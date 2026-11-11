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

// Consumer consumes notification payloads from RabbitMQ with bounded retries and DLQ routing.
type Consumer struct {
	url   string
	queue string
	conn  *amqp.Connection
	ch    *amqp.Channel
	log   *logger.Logger
}

// NewConsumer dials RabbitMQ and prepares queue topology for consuming messages.
func NewConsumer(url, queue string, log *logger.Logger) (*Consumer, error) {
	c := &Consumer{
		url:   url,
		queue: queue,
		log:   log,
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Consumer) connect() error {
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
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("rabbitmq set qos: %w", err)
	}

	if err := declareTopology(ch, c.queue); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("rabbitmq declare topology: %w", err)
	}

	c.conn = conn
	c.ch = ch
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

// Consume blocks until ctx is cancelled, invoking handler for each delivered payload.
func (c *Consumer) Consume(ctx context.Context, handler func(payload *domain.NotificationPayload) error) error {
	for {
		connClose := c.conn.NotifyClose(make(chan *amqp.Error, 1))

		deliveries, err := c.ch.Consume(c.queue, "", false, false, false, false, nil)
		if err != nil {
			c.log.Error(err, map[string]string{"operation": "rabbitmq_consume"})
			if err := c.reconnect(ctx); err != nil {
				return err
			}
			continue
		}

		c.log.Info("RabbitMQ consumer started", map[string]string{"queue": c.queue})

	loop:
		for {
			select {
			case <-ctx.Done():
				return nil

			case amqpErr := <-connClose:
				if amqpErr != nil {
					c.log.Error(amqpErr, map[string]string{"operation": "consumer_connection_closed"})
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

				var payload domain.NotificationPayload
				if err := json.Unmarshal(delivery.Body, &payload); err != nil {
					c.log.Error(err, map[string]string{"operation": "unmarshal_payload"})
					_ = delivery.Nack(false, false) // Discard malformed message.
					continue
				}

				if err := handler(&payload); err != nil {
					c.log.Error(err, map[string]string{"operation": "handle_notification"})
					if retryErr := c.retryOrDeadLetter(ctx, delivery); retryErr != nil {
						c.log.Error(retryErr, map[string]string{"operation": "retry_or_dead_letter"})
						_ = delivery.Nack(false, true)
						continue
					}
					_ = delivery.Ack(false)
					continue
				}

				_ = delivery.Ack(false)
			}
		}
	}
}

func (c *Consumer) retryOrDeadLetter(ctx context.Context, delivery amqp.Delivery) error {
	headers := copyHeaders(delivery.Headers)
	attempt := retryCount(headers) + 1
	headers[retryCountHeader] = int32(attempt)

	routingKey := retryQueueName(c.queue)
	if attempt > maxRetryCount {
		routingKey = deadLetterQueueName(c.queue)
	}

	return c.ch.PublishWithContext(ctx, "", routingKey, false, false, amqp.Publishing{
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
}

// Close closes the AMQP channel and connection.
func (c *Consumer) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
