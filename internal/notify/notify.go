// Package notify delivers alert messages to the station owner.
package notify

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Message is one alert.
type Message struct {
	Event    string // "down", "recovered" or "test"
	Callsign string
	Subject  string
	Body     string
}

// Notifier delivers a Message over one channel.
type Notifier interface {
	Name() string
	Send(ctx context.Context, m Message) error
}

// Dispatch sends m to every notifier. A failing notifier is retried up to
// attempts times total with doubling delay starting at baseDelay, and never
// blocks the others. The returned error names each notifier that gave up.
func Dispatch(ctx context.Context, ns []Notifier, m Message, attempts int, baseDelay time.Duration) error {
	if attempts < 1 {
		attempts = 1
	}
	var errs []error
	for _, n := range ns {
		if err := sendWithRetry(ctx, n, m, attempts, baseDelay); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", n.Name(), err))
		}
	}
	return errors.Join(errs...)
}

func sendWithRetry(ctx context.Context, n Notifier, m Message, attempts int, delay time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = n.Send(ctx, m); err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return errors.Join(err, ctx.Err())
		case <-time.After(delay):
		}
		delay *= 2
	}
	return err
}
