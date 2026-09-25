// Package publisher sends post domain events to the notification
// service. The RabbitMQ broker is the primary transport; the HTTP
// publisher remains as a fallback for broker-less environments.
package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"post/internal/trace"
)

type HTTPPublisher struct {
	url    string
	client *http.Client
}

func NewHTTP(url string) *HTTPPublisher {
	return &HTTPPublisher{
		url:    url,
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

func (p *HTTPPublisher) Publish(ctx context.Context, eventType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url+"/events/"+eventType, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s := trace.SpanFrom(ctx); s != nil {
		req.Header.Set("traceparent", s.Traceparent())
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification service returned %d", resp.StatusCode)
	}
	return nil
}

type NopPublisher struct{}

func (NopPublisher) Publish(ctx context.Context, eventType string, payload any) error {
	return nil
}
