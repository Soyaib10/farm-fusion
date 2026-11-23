package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultPublishTimeout = 15 * time.Second
	defaultMaxOutage      = 10 * time.Minute
	maxReconnectDelay     = 30 * time.Second
)

// Publisher publishes durable notification payloads to RabbitMQ with broker confirms. It reconnects
// for as long as the process runs; if the broker stays unreachable longer than the allowed outage,
// Failed() fires so the process can exit and be restarted by its supervisor.
type Publisher struct {
	url            string
	queue          string
	publishTimeout time.Duration
	maxOutage      time.Duration

	mu        sync.Mutex
	conn      *amqp.Connection
	ch        *amqp.Channel
	done      chan struct{}
	closeOnce sync.Once
	failed    chan error
	failOnce  sync.Once
	log       *logger.Logger
}

// NewPublisher connects to RabbitMQ, declares queue topology, and starts background reconnect monitoring.
func NewPublisher(url, queue string, log *logger.Logger) (*Publisher, error) {
	p := &Publisher{
		url:            url,
		queue:          queue,
		publishTimeout: defaultPublishTimeout,
		maxOutage:      defaultMaxOutage,
		done:           make(chan struct{}),
		failed:         make(chan error, 1),
		log:            log,
	}
	if err := p.connect(); err != nil {
		return nil, err
	}
	go p.watchConnection()
	return p, nil
}

// Failed delivers one error once publishing has been impossible for longer than the allowed outage.
func (p *Publisher) Failed() <-chan error {
	return p.failed
}

// connect dials a new connection and replaces the current one, closing the old connection first.
// Callers other than NewPublisher must hold p.mu.
func (p *Publisher) connect() error {
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn, p.ch = nil, nil
	}

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
		_ = conn.Close()
		return fmt.Errorf("rabbitmq enable confirms: %w", err)
	}

	if err := declareTopology(ch, p.queue); err != nil {
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
		conn, ch := p.conn, p.ch
		if conn == nil || ch == nil {
			p.mu.Unlock()
			return
		}
		connClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		chClose := ch.NotifyClose(make(chan *amqp.Error, 1))
		p.mu.Unlock()

		select {
		case <-p.done:
			return
		case reason := <-connClose:
			p.log.Error(closeReason(reason), map[string]string{"operation": "publisher_connection_closed"})
		case reason := <-chClose:
			p.log.Error(closeReason(reason), map[string]string{"operation": "publisher_channel_closed"})
		}

		if !p.reconnect() {
			return
		}
	}
}

// reconnect retries until it succeeds or the publisher is closed.
func (p *Publisher) reconnect() bool {
	downSince := time.Now()
	for attempt := 1; ; attempt++ {
		select {
		case <-p.done:
			return false
		case <-time.After(reconnectDelay(attempt)):
		}

		p.log.Info("RabbitMQ publisher reconnecting", map[string]string{"attempt": strconv.Itoa(attempt)})

		p.mu.Lock()
		select {
		case <-p.done:
			p.mu.Unlock()
			return false
		default:
		}
		err := p.connect()
		p.mu.Unlock()

		if err == nil {
			p.log.Info("RabbitMQ publisher reconnected", nil)
			return true
		}
		p.log.Error(err, map[string]string{"operation": "rabbitmq_publisher_reconnect"})

		if outage := time.Since(downSince); outage > p.maxOutage {
			p.fail(fmt.Errorf("rabbitmq publisher unavailable for %s", outage.Round(time.Second)))
		}
	}
}

func (p *Publisher) fail(err error) {
	p.failOnce.Do(func() {
		p.failed <- err
	})
}

// reconnectDelay is exponential backoff capped at maxReconnectDelay, with jitter in [d/2, d).
func reconnectDelay(attempt int) time.Duration {
	d := maxReconnectDelay
	if attempt < 6 {
		d = time.Second << (attempt - 1)
	}
	if d > maxReconnectDelay {
		d = maxReconnectDelay
	}
	half := d / 2
	return half + rand.N(half)
}

func closeReason(reason *amqp.Error) error {
	if reason == nil {
		return errors.New("closed without a reason")
	}
	return reason
}

// Publish serializes payload as JSON and waits for broker confirmation. Each call is bounded by
// the publish timeout in addition to ctx.
func (p *Publisher) Publish(ctx context.Context, payload *domain.NotificationPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()

	p.mu.Lock()
	if p.ch == nil {
		p.mu.Unlock()
		return errors.New("rabbitmq publisher is not connected")
	}
	confirm, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, "", p.queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    payload.NotificationID.String(),
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
		if p.conn != nil {
			_ = p.conn.Close()
			p.conn, p.ch = nil, nil
		}
	})
}
