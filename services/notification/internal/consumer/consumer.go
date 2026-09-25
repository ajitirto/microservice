// Package consumer runs the RabbitMQ event consumer for post domain
// events. Failed deliveries go to a retry queue that dead-letters back
// to the main queue after a TTL; deliveries that exhaust the retry
// budget land in the dead letter queue instead of being dropped.
package consumer

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"notification/internal/service"
	"notification/internal/trace"
)

const (
	exchange     = "post.events"
	mainQueue    = "post.notifications"
	retryQueue   = "post.notifications.retry"
	deadQueue    = "post.notifications.dlq"
	retryHeader  = "x-retry-count"
	defaultTTL   = 5 * time.Second
	defaultRetry = 3
)

type Config struct {
	URL        string
	MaxRetries int
	RetryTTL   time.Duration
	Logger     *slog.Logger
}

type Consumer struct {
	cfg        Config
	svc        *service.Service
	logger     *slog.Logger
	maxRetries int
	retryTTL   time.Duration

	conn *amqp.Connection
	ch   *amqp.Channel
}

func New(cfg Config, svc *service.Service) *Consumer {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultRetry
	}
	if cfg.RetryTTL <= 0 {
		cfg.RetryTTL = defaultTTL
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Consumer{
		cfg:        cfg,
		svc:        svc,
		logger:     cfg.Logger,
		maxRetries: cfg.MaxRetries,
		retryTTL:   cfg.RetryTTL,
	}
}

// Start blocks consuming events until ctx is cancelled, reconnecting
// to the broker whenever the connection drops.
func (c *Consumer) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			c.close()
			return nil
		default:
		}

		if err := c.connectAndConsume(ctx); err != nil {
			c.logger.Warn("event consumer error, reconnecting", "error", err)
			c.close()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
		}
	}
}

func (c *Consumer) connectAndConsume(ctx context.Context) error {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := amqp.DialConfig(c.cfg.URL, amqp.Config{Dial: dialer.Dial})
	if err != nil {
		return err
	}
	c.conn = conn

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	c.ch = ch
	return c.consume(ctx)
}

func (c *Consumer) consume(ctx context.Context) error {
	ch := c.ch

	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(mainQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(mainQueue, "post_created", exchange, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(mainQueue, "post_liked", exchange, false, nil); err != nil {
		return err
	}
	retryArgs := amqp.Table{
		"x-message-ttl":             int32(c.retryTTL / time.Millisecond),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": mainQueue,
	}
	if _, err := ch.QueueDeclare(retryQueue, true, false, false, false, retryArgs); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(deadQueue, true, false, false, false, nil); err != nil {
		return err
	}

	deliveries, err := ch.Consume(mainQueue, "post-notifications", false, false, false, false, nil)
	if err != nil {
		return err
	}

	c.logger.Info("event consumer started",
		"queue", mainQueue, "retry_ttl", c.retryTTL, "max_retries", c.maxRetries)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ch.NotifyClose(make(chan *amqp.Error, 1)):
			return errors.New("consumer channel closed")
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("consumer delivery channel closed")
			}
			c.handle(ctx, d)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) {
	parent, _ := trace.ParseTraceparent(headerString(d.Headers, "traceparent"))
	ctx = trace.WithSpan(ctx, trace.New(parent))
	span := trace.SpanFrom(ctx)

	if err := c.handleOne(ctx, d); err != nil {
		c.logger.Warn("event processing failed",
			"routing_key", d.RoutingKey, "error", err,
			"trace_id", span.TraceID, "span_id", span.SpanID)
	} else {
		c.logger.Info("event processed",
			"routing_key", d.RoutingKey,
			"trace_id", span.TraceID, "span_id", span.SpanID)
	}

	select {
	case <-ctx.Done():
		_ = d.Nack(false, true)
	default:
		_ = d.Ack(false)
	}
}

func headerString(headers amqp.Table, key string) string {
	v, ok := headers[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (c *Consumer) handleOne(ctx context.Context, d amqp.Delivery) error {
	if _, err := c.svc.Ingest(ctx, d.RoutingKey, d.Body); err == nil {
		return nil
	} else {
		queue := deadQueue
		if ShouldRetry(retryCount(d.Headers), c.maxRetries) {
			queue = retryQueue
		}
		if rerr := c.republish(ctx, d, queue); rerr != nil {
			c.logger.Error("event redelivery failed", "queue", queue, "error", rerr)
		}
		return err
	}
}

func (c *Consumer) republish(ctx context.Context, d amqp.Delivery, queue string) error {
	headers := amqp.Table{}
	for k, v := range d.Headers {
		headers[k] = v
	}
	headers[retryHeader] = retryCount(d.Headers) + 1

	return c.ch.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  d.ContentType,
		DeliveryMode: d.DeliveryMode,
		Timestamp:    time.Now().UTC(),
		Headers:      headers,
		Body:         d.Body,
	})
}

func (c *Consumer) close() {
	if c.ch != nil {
		_ = c.ch.Close()
		c.ch = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func retryCount(headers amqp.Table) int {
	v, ok := headers[retryHeader]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int32:
		return int(n)
	case int64:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

// ShouldRetry reports whether a delivery with the given retry count may
// be attempted again under the retry budget.
func ShouldRetry(count, maxRetries int) bool {
	return count < maxRetries
}
