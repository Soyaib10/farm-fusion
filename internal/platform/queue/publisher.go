package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes durable notification payloads to RabbitMQ with broker confirms.
type Publisher struct {
	url       string
	queue     string
	conn      *amqp.Connection
	ch        *amqp.Channel
	mu        sync.Mutex
	done      chan struct{}
	closeOnce sync.Once
	log       *logger.Logger
}

// NewPublisher connects to RabbitMQ, declares queue topology, and starts background reconnect monitoring.
func NewPublisher(url, queue string, log *logger.Logger) (*Publisher, error) {
	p := &Publisher{
		url:   url,
		queue: queue,
		done:  make(chan struct{}),
		log:   log,
	}
	if err := p.connect(); err != nil {
		return nil, err
	}
	go p.watchConnection()
	return p, nil
}

func (p *Publisher) connect() error {
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return errors.New("rabbitmq dial failed")
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("rabbitmq open channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("rabbitmq enable confirms: %w", err)
	}

	if err := declareTopology(ch, p.queue); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("rabbitmq declare topology: %w", err)
	}

	p.conn = conn
	p.ch = ch
	return nil
}

func (p *Publisher) watchConnection() {
	for {
		p.mu.Lock()
		if p.conn == nil {
			p.mu.Unlock()
			return
		}
		notifyClose := p.conn.NotifyClose(make(chan *amqp.Error, 1))
		p.mu.Unlock()

		select {
		case <-p.done:
			return
		case reason, ok := <-notifyClose:
			if !ok {
				return
			}
			p.log.Error(reason, map[string]string{"operation": "publisher_connection_closed"})
		}

		reconnected := false
		for attempt := 1; attempt <= 5; attempt++ {
			select {
			case <-p.done:
				return
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}

			p.log.Info("RabbitMQ publisher reconnecting", map[string]string{"attempt": strconv.Itoa(attempt)})

			p.mu.Lock()
			select {
			case <-p.done:
				p.mu.Unlock()
				return
			default:
			}
			err := p.connect()
			p.mu.Unlock()

			if err != nil {
				p.log.Error(err, map[string]string{"operation": "rabbitmq_publisher_reconnect"})
				continue
			}

			p.log.Info("RabbitMQ publisher reconnected", nil)
			reconnected = true
			break
		}

		if !reconnected {
			p.log.Error(errors.New("rabbitmq publisher exhausted reconnect attempts"), nil)
			return
		}
	}
}

// Publish serializes payload as JSON and waits for broker confirmation.
func (p *Publisher) Publish(ctx context.Context, payload *domain.NotificationPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	p.mu.Lock()
	if p.ch == nil {
		p.mu.Unlock()
		return errors.New("rabbitmq publisher is closed")
	}
	confirm, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, "", p.queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
	p.mu.Unlock()

	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	ok, err := confirm.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait broker confirm: %w", err)
	}
	if !ok {
		return errors.New("message nacked by broker")
	}
	return nil
}

// Close stops connection monitoring and closes the AMQP channel and connection.
func (p *Publisher) Close() {
	p.closeOnce.Do(func() {
		close(p.done)
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.ch != nil {
			_ = p.ch.Close()
			p.ch = nil
		}
		if p.conn != nil {
			_ = p.conn.Close()
			p.conn = nil
		}
	})
}
