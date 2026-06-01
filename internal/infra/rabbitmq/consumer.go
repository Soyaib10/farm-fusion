package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/notification"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	url    string
	queue  string
	conn   *amqp.Connection
	ch     *amqp.Channel
	logger *logger.Logger
}

func NewConsumer(url, queue string, logger *logger.Logger) (notification.QueueConsumer, error) {
	c := &Consumer{url: url, queue: queue, logger: logger}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Consumer) connect() error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("rabbitmq open channel: %w", err)
	}

	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq set qos: %w", err)
	}

	if err := declareTopology(ch, c.queue); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq declare topology: %w", err)
	}

	c.conn = conn
	c.ch = ch
	return nil
}

func (c *Consumer) reconnect(ctx context.Context) error {
	for attempt := 1; attempt <= 5; attempt++ {
		c.logger.PrintInfo("RabbitMQ consumer reconnecting", map[string]string{"attempt": fmt.Sprintf("%d", attempt)})

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 2 * time.Second):
		}

		if err := c.connect(); err != nil {
			c.logger.PrintError(err, map[string]string{"operation": "rabbitmq_consumer_reconnect"})
			continue
		}
		c.logger.PrintInfo("RabbitMQ consumer reconnected", nil)
		return nil
	}
	return fmt.Errorf("rabbitmq consumer: exhausted reconnect attempts")
}

func (c *Consumer) Consume(ctx context.Context, handler func(payload *domain.NotificationPayload) error) error {
	for {
		connClose := c.conn.NotifyClose(make(chan *amqp.Error, 1))

		deliveries, err := c.ch.Consume(c.queue, "", false, false, false, false, nil)
		if err != nil {
			c.logger.PrintError(err, map[string]string{"operation": "rabbitmq_consume"})
			if err := c.reconnect(ctx); err != nil {
				return err
			}
			continue
		}

		c.logger.PrintInfo("RabbitMQ consumer started", map[string]string{"queue": c.queue})

	loop:
		for {
			select {
			case <-ctx.Done():
				return nil

			case amqpErr := <-connClose:
				if amqpErr != nil {
					c.logger.PrintError(amqpErr, map[string]string{"operation": "consumer_connection_closed"})
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
					c.logger.PrintError(err, map[string]string{"operation": "unmarshal_payload"})
					delivery.Nack(false, false) // discard malformed
					continue
				}

				if err := handler(&payload); err != nil {
					c.logger.PrintError(err, map[string]string{"operation": "handle_notification"})
					if retryErr := c.retryOrDeadLetter(ctx, delivery); retryErr != nil {
						c.logger.PrintError(retryErr, map[string]string{"operation": "retry_or_dead_letter"})
						delivery.Nack(false, true)
						continue
					}
					delivery.Ack(false)
					continue
				}

				delivery.Ack(false)
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

func (c *Consumer) Close() {
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
