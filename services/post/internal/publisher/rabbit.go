package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const eventExchange = "post.events"

// RabbitPublisher publishes post domain events to the RabbitMQ
// exchange post.events (topic) using publisher confirms, so a
// published event is acknowledged only when the broker has it.
type RabbitPublisher struct {
	url    string
	logger *slog.Logger

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewRabbit(url string, logger *slog.Logger) *RabbitPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &RabbitPublisher{url: url, logger: logger}
}

func (p *RabbitPublisher) Publish(ctx context.Context, eventType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureConn(); err != nil {
		return err
	}

	conf, err := p.ch.PublishWithDeferredConfirmWithContext(
		ctx,
		eventExchange,
		eventType,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         body,
		},
	)
	if err != nil {
		p.closeConn()
		return err
	}

	select {
	case <-conf.Done():
		if !conf.Acked() {
			return fmt.Errorf("broker did not confirm event %s", eventType)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *RabbitPublisher) ensureConn() error {
	if p.conn != nil && !p.conn.IsClosed() {
		return nil
	}

	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := amqp.DialConfig(p.url, amqp.Config{Dial: dialer.Dial})
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}
	if err := ch.ExchangeDeclare(eventExchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	p.conn = conn
	p.ch = ch
	return nil
}

func (p *RabbitPublisher) closeConn() {
	if p.ch != nil {
		_ = p.ch.Close()
		p.ch = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}
