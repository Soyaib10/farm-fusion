package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultHandlerTimeout = 2 * time.Minute
	republishTimeout      = 15 * time.Second
	dlqCheckInterval      = 5 * time.Minute
)

// Handler processes one notification. ctx is bounded per message and is not cancelled by shutdown,
// so an in-flight delivery can finish.
type Handler func(ctx context.Context, payload *domain.NotificationPayload) error

// Consumer consumes notification payloads from RabbitMQ with bounded retries and DLQ routing.
// Deliveries are consumed on one channel; retry and dead-letter copies are published on a second,
// confirm-mode channel, and the original is acked only after the broker confirms the copy.
type Consumer struct {
	url            string
	queue          string
	conn           *amqp.Connection
	ch             *amqp.Channel
	pubCh          *amqp.Channel
	returns        chan amqp.Return
	log            *logger.Logger
	handlerTimeout time.Duration
}

// NewConsumer dials RabbitMQ and prepares queue topology for consuming messages.
func NewConsumer(url, queue string, log *logger.Logger) (*Consumer, error) {
	c := &Consumer{
		url:            url,
		queue:          queue,
		log:            log,
		handlerTimeout: defaultHandlerTimeout,
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Consumer) connect() error {
	c.closeConn()

	conn, err := amqp.Dial(c.url)
	if err != nil {
		return errors.New("rabbitmq dial failed")
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("rabbitmq open channel: %w", err)
	}
	if err := ch.Qos(1, 0, false); err != nil {
		_ = conn.Close()
		return fmt.Errorf("rabbitmq set qos: %w", err)
	}
	if err := declareTopology(ch, c.queue); err != nil {
		_ = conn.Close()
		return fmt.Errorf("rabbitmq declare topology: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("rabbitmq open publish channel: %w", err)
	}
	if err := pubCh.Confirm(false); err != nil {
		_ = conn.Close()
		return fmt.Errorf("rabbitmq enable confirms: %w", err)
	}

	c.conn = conn
	c.ch = ch
	c.pubCh = pubCh
	c.returns = pubCh.NotifyReturn(make(chan amqp.Return, 1))
	return nil
}

func (c *Consumer) reconnect(ctx context.Context) error {
	for attempt := 1; attempt <= 5; attempt++ {
		c.log.Info("RabbitMQ consumer reconnecting", map[string]string{"attempt": strconv.Itoa(attempt)})

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 2 * time.Second):
		}

		if err := c.connect(); err != nil {
			c.log.Error(err, map[string]string{"operation": "rabbitmq_consumer_reconnect"})
			continue
		}
		c.log.Info("RabbitMQ consumer reconnected", nil)
		return nil
	}
	return errors.New("rabbitmq consumer exhausted reconnect attempts")
}

// Consume blocks until ctx is cancelled, invoking handler for each delivered payload. Shutdown waits
// for the in-flight message to finish before returning.
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	dlqTicker := time.NewTicker(dlqCheckInterval)
	defer dlqTicker.Stop()

	for {
		connClose := c.conn.NotifyClose(make(chan *amqp.Error, 1))
		pubClose := c.pubCh.NotifyClose(make(chan *amqp.Error, 1))

		deliveries, err := c.ch.Consume(c.queue, "", false, false, false, false, nil)
		if err != nil {
			c.log.Error(err, map[string]string{"operation": "rabbitmq_consume"})
			if err := c.reconnect(ctx); err != nil {
				return err
			}
			continue
		}

		c.log.Info("RabbitMQ consumer started", map[string]string{"queue": c.queue})
		c.checkDeadLetterDepth()

	loop:
		for {
			select {
			case <-ctx.Done():
				return nil

			case <-dlqTicker.C:
				c.checkDeadLetterDepth()

			case amqpErr := <-connClose:
				if amqpErr != nil {
					c.log.Error(amqpErr, map[string]string{"operation": "consumer_connection_closed"})
				}
				if err := c.reconnect(ctx); err != nil {
					return err
				}
				break loop

			case amqpErr := <-pubClose:
				if amqpErr != nil {
					c.log.Error(amqpErr, map[string]string{"operation": "consumer_publish_channel_closed"})
				}
				if err := c.reconnect(ctx); err != nil {
					return err
				}
				break loop

			case delivery, ok := <-deliveries:
				if !ok {
					if err := c.reconnect(ctx); err != nil {
						return err
					}
					break loop
				}
				c.handleDelivery(delivery, handler)
			}
		}
	}
}

func (c *Consumer) handleDelivery(delivery amqp.Delivery, handler Handler) {
	var payload domain.NotificationPayload
	if err := json.Unmarshal(delivery.Body, &payload); err != nil {
		c.log.Error(err, map[string]string{"operation": "unmarshal_payload"})
		c.reject(delivery) // Dead-lettered to the DLQ by the main queue's arguments.
		return
	}

	msgCtx, cancel := context.WithTimeout(context.Background(), c.handlerTimeout)
	err := handler(msgCtx, &payload)
	cancel()

	if err == nil {
		c.ack(delivery)
		return
	}

	c.log.Error(err, map[string]string{
		"operation":       "handle_notification",
		"notification_id": payload.NotificationID.String(),
	})
	if err := c.retryOrDeadLetter(delivery); err != nil {
		c.log.Error(err, map[string]string{"operation": "retry_or_dead_letter"})
		c.reject(delivery)
		return
	}
	c.ack(delivery)
}

// retryOrDeadLetter republishes a copy to the retry queue (or the DLQ once retries are used up)
// and waits for the broker to confirm it.
func (c *Consumer) retryOrDeadLetter(delivery amqp.Delivery) error {
	headers := copyHeaders(delivery.Headers)
	count, known := retryCount(headers)
	attempt := count + 1
	headers[retryCountHeader] = int32(attempt)

	routingKey := retryQueueName(c.queue)
	if !known {
		c.log.Error(fmt.Errorf("unrecognised %s header type %T", retryCountHeader, delivery.Headers[retryCountHeader]),
			map[string]string{"operation": "retry_count"})
		routingKey = deadLetterQueueName(c.queue)
	} else if attempt > maxRetryCount {
		routingKey = deadLetterQueueName(c.queue)
	}

	ctx, cancel := context.WithTimeout(context.Background(), republishTimeout)
	defer cancel()

	confirm, err := c.pubCh.PublishWithDeferredConfirmWithContext(ctx, "", routingKey, true, false, amqp.Publishing{
		ContentType:     delivery.ContentType,
		ContentEncoding: delivery.ContentEncoding,
		DeliveryMode:    amqp.Persistent,
		Priority:        delivery.Priority,
		CorrelationId:   delivery.CorrelationId,
		ReplyTo:         delivery.ReplyTo,
		Expiration:      delivery.Expiration,
		MessageId:       delivery.MessageId,
		Timestamp:       time.Now().UTC(),
		Type:            delivery.Type,
		UserId:          delivery.UserId,
		AppId:           delivery.AppId,
		Headers:         headers,
		Body:            delivery.Body,
	})
	if err != nil {
		return fmt.Errorf("republish to %s: %w", routingKey, err)
	}
	ok, err := confirm.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait republish confirm: %w", err)
	}
	if !ok {
		return fmt.Errorf("republish to %s nacked by broker", routingKey)
	}
	// The broker sends basic.return before the confirm, so an unroutable copy is already queued here.
	select {
	case ret := <-c.returns:
		return fmt.Errorf("republish to %s returned as unroutable: %s", routingKey, ret.ReplyText)
	default:
	}
	return nil
}

func (c *Consumer) ack(delivery amqp.Delivery) {
	if err := delivery.Ack(false); err != nil {
		c.log.Error(err, map[string]string{"operation": "ack_delivery"})
	}
}

// reject dead-letters the delivery into the DLQ without requeueing, so a broken message or channel
// cannot spin in a tight redelivery loop.
func (c *Consumer) reject(delivery amqp.Delivery) {
	if err := delivery.Nack(false, false); err != nil {
		c.log.Error(err, map[string]string{"operation": "nack_delivery"})
	}
}

// checkDeadLetterDepth logs an error while messages are waiting in the DLQ; nothing consumes it
// automatically, so those alerts need an operator to inspect and replay them.
func (c *Consumer) checkDeadLetterDepth() {
	q, err := c.ch.QueueDeclarePassive(deadLetterQueueName(c.queue), true, false, false, false, nil)
	if err != nil {
		c.log.Error(err, map[string]string{"operation": "inspect_dead_letter_queue"})
		return
	}
	if q.Messages > 0 {
		c.log.Error(errors.New("weather notifications are waiting in the dead-letter queue"), map[string]string{
			"operation": "dead_letter_queue_depth",
			"queue":     q.Name,
			"messages":  strconv.Itoa(q.Messages),
		})
	}
}

func (c *Consumer) closeConn() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn, c.ch, c.pubCh = nil, nil, nil
}

// Close closes the AMQP channels and connection.
func (c *Consumer) Close() {
	c.closeConn()
}
