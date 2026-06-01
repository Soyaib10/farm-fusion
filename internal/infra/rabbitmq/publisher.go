package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/notification"
	"github.com/Soyaib10/farm-fusion/pkg/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	url    string
	queue  string
	conn   *amqp.Connection
	ch     *amqp.Channel
	mu     sync.Mutex
	logger *logger.Logger
}

func NewPublisher(url, queue string, logger *logger.Logger) (notification.QueuePublisher, error) {
	p := &Publisher{url: url, queue: queue, logger: logger}
	if err := p.connect(); err != nil {
		return nil, err
	}
	go p.watchConnection()
	return p, nil
}

func (p *Publisher) connect() error {
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("rabbitmq open channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq enable confirms: %w", err)
	}

	if err := declareTopology(ch, p.queue); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq declare topology: %w", err)
	}

	p.conn = conn
	p.ch = ch
	return nil
}

// watchConnection loops: registers NotifyClose, waits for disconnect, reconnects, repeats.
func (p *Publisher) watchConnection() {
	for {
		p.mu.Lock()
		notifyClose := p.conn.NotifyClose(make(chan *amqp.Error, 1))
		p.mu.Unlock()

		reason, ok := <-notifyClose
		if !ok {
			return // closed intentionally via Close()
		}
		p.logger.PrintError(reason, map[string]string{"operation": "publisher_connection_closed"})

		p.mu.Lock()
		reconnected := false
		for attempt := 1; attempt <= 5; attempt++ {
			p.logger.PrintInfo("RabbitMQ publisher reconnecting", map[string]string{"attempt": fmt.Sprintf("%d", attempt)})
			if err := p.connect(); err != nil {
				p.logger.PrintError(err, map[string]string{"operation": "rabbitmq_publisher_reconnect"})
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
			p.logger.PrintInfo("RabbitMQ publisher reconnected", nil)
			reconnected = true
			break
		}
		p.mu.Unlock()

		if !reconnected {
			p.logger.PrintError(fmt.Errorf("rabbitmq publisher: exhausted reconnect attempts"), nil)
			return
		}
	}
}

func (p *Publisher) Publish(ctx context.Context, payload *domain.NotificationPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	p.mu.Lock()
	confirm, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, "", p.queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
	p.mu.Unlock()

	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	ok, err := confirm.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("confirm wait: %w", err)
	}
	if !ok {
		return fmt.Errorf("message nacked by broker")
	}
	return nil
}

func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil {
		p.ch.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}
