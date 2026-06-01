package rabbitmq

import (
	"fmt"

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

	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare main queue: %w", err)
	}
	return nil
}

func copyHeaders(headers amqp.Table) amqp.Table {
	copied := amqp.Table{}
	for key, value := range headers {
		copied[key] = value
	}
	return copied
}

func retryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	switch value := headers[retryCountHeader].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
