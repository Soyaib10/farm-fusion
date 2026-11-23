package queue

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRetryCountAcceptsAMQPIntegerTypes(t *testing.T) {
	for _, v := range []any{int32(2), int64(2), uint32(2), int16(2), int8(2), uint8(2), float64(2), "2"} {
		n, ok := retryCount(amqp.Table{retryCountHeader: v})
		if !ok || n != 2 {
			t.Fatalf("retryCount(%T) = %d, %v; want 2, true", v, n, ok)
		}
	}

	if n, ok := retryCount(nil); !ok || n != 0 {
		t.Fatalf("missing header = %d, %v; want 0, true", n, ok)
	}
	if _, ok := retryCount(amqp.Table{retryCountHeader: []byte("x")}); ok {
		t.Fatal("unknown header type must report ok=false so the message is dead-lettered")
	}
	if _, ok := retryCount(amqp.Table{retryCountHeader: "many"}); ok {
		t.Fatal("non-numeric string must report ok=false")
	}
}
