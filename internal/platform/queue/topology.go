package queue

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	retryCountHeader = "x-retry-count"
	maxRetryCount    = 3
	retryDelayMS     = int32(5 * 60 * 1000)
)

func retryQueueName(queue string) string {
	return queue + ".retry"
}

func deadLetterQueueName(queue string) string {
	return queue + ".dlq"
}

// declareTopology declares the main queue, a delayed retry queue that dead-letters back into it,
// and a DLQ. The main queue dead-letters rejected messages (Nack without requeue) into the DLQ, so
// nothing the worker gives up on is dropped.
//
// Changing queue arguments requires deleting an existing queue first; RabbitMQ refuses to redeclare
// a queue with different arguments (PRECONDITION_FAILED).
func declareTopology(ch *amqp.Channel, queue string) error {
	if _, err := ch.QueueDeclare(deadLetterQueueName(queue), true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dead-letter queue: %w", err)
	}

	retryArgs := amqp.Table{
		"x-message-ttl":             retryDelayMS,
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": queue,
	}
	if _, err := ch.QueueDeclare(retryQueueName(queue), true, false, false, false, retryArgs); err != nil {
		return fmt.Errorf("declare retry queue: %w", err)
	}

	mainArgs := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": deadLetterQueueName(queue),
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, mainArgs); err != nil {
		return fmt.Errorf("declare main queue: %w", err)
	}
	return nil
}

func copyHeaders(headers amqp.Table) amqp.Table {
	copied := make(amqp.Table, len(headers))
	for k, v := range headers {
		copied[k] = v
	}
	return copied
}

// retryCount reads the retry header. ok is false when the header holds a value of an unknown type,
// in which case the caller must dead-letter instead of retrying with a reset counter.
func retryCount(headers amqp.Table) (count int, ok bool) {
	raw, present := headers[retryCountHeader]
	if !present || raw == nil {
		return 0, true
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		if v > math.MaxInt32 {
			return 0, false
		}
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		return n, err == nil
	default:
		return 0, false
	}
}
