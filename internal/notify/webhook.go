package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Webhook POSTs a JSON body to a URL. The body carries the text under both
// "content" (Discord) and "text" (Slack, Mattermost, Rocket.Chat, ...), plus
// structured fields for generic receivers.
type Webhook struct {
	url    string
	client *http.Client
}

// NewWebhook returns a Webhook notifier for url.
func NewWebhook(url string) *Webhook {
	return &Webhook{url: url, client: &http.Client{Timeout: 15 * time.Second}}
}

func (w *Webhook) Name() string { return "webhook" }

func (w *Webhook) Send(ctx context.Context, m Message) error {
	text := m.Subject + "\n" + m.Body
	payload, err := json.Marshal(map[string]string{
		"content":  text,
		"text":     text,
		"subject":  m.Subject,
		"body":     m.Body,
		"event":    m.Event,
		"callsign": m.Callsign,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(payload))
	if err != nil {
		return redactURL(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return redactURL(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("http %d: %s", resp.StatusCode, snippet)
	}
	return nil
}

// redactURL drops the URL from client errors: webhook URLs are secrets (the
// token is in the path) and errors end up in logs.
func redactURL(err error) error {
	if ue, ok := err.(interface{ Unwrap() error }); ok && ue.Unwrap() != nil {
		return fmt.Errorf("request failed: %w", ue.Unwrap())
	}
	return fmt.Errorf("request failed")
}
