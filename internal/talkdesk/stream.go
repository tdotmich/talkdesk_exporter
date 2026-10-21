package talkdesk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	initialRetryDelay = time.Second
	maxRetryDelay     = 30 * time.Second

	// Talkdesk sends each query at least once every 60 seconds, also when the value
	// does not change. If we get nothing for longer than this, the connection is dead.
	streamIdleTimeout = 2 * time.Minute
)

// StreamHandler receives what happens on the stream. The metrics package implements it.
type StreamHandler interface {
	HandleEvent(streamEvent *StreamEvent)
	HandleConnectionChange(connected bool)
}

func (client *Client) ConnectStream(ctx context.Context, subscription *LiveSubscription) (io.ReadCloser, error) {
	// The stream URL contains a token for this subscription, so we use it exactly as Talkdesk gives it.
	streamURL := subscription.Links.Stream.Href

	if streamURL == "" {
		return nil, fmt.Errorf("subscription does not contain a stream url")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)

	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}

	request.Header.Set("Accept", "text/event-stream")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connect to subscription stream: %w", removeURLFromError(err))
	}

	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()

		return nil, fmt.Errorf(
			"subscription stream returned HTTP status %s: %s",
			response.Status,
			readErrorBody(response),
		)
	}

	return response.Body, nil
}

// RunStream keeps the exporter connected to Talkdesk Live until ctx is cancelled.
// When the stream stops, it waits, then connects again. The wait doubles after each
// failure (1s, 2s, 4s ... 30s), so we do not flood Talkdesk with requests.
func (client *Client) RunStream(
	ctx context.Context,
	liveQueries []LiveQuery,
	handler StreamHandler,
) error {
	retryDelay := initialRetryDelay

	for {
		receivedEvents, err := client.streamOnce(ctx, liveQueries, handler)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// The connection worked for a time, so start again from the shortest wait.
		if receivedEvents {
			retryDelay = initialRetryDelay
		}

		log.Printf("Talkdesk Live stream stopped, reconnecting in %s: %v", retryDelay, err)

		err = waitForRetry(ctx, retryDelay)
		if err != nil {
			return err
		}

		retryDelay = min(retryDelay*2, maxRetryDelay)
	}
}

// streamOnce creates a new subscription, connects to its stream and reads events
// until the stream stops. It returns true if at least one event arrived.
//
// We create a new subscription for each connection. The stream URL contains a token
// that can expire, and a new subscription always gives us a new token.
func (client *Client) streamOnce(
	ctx context.Context,
	liveQueries []LiveQuery,
	handler StreamHandler,
) (bool, error) {
	receivedEvents := false

	subscription, err := client.CreateSubscription(ctx, liveQueries)
	if err != nil {
		return receivedEvents, err
	}

	log.Printf("Created Talkdesk Live subscription: id=%s", subscription.ID)

	// If no event arrives before the idle timer fires, the timer cancels streamCtx.
	// This closes the connection and makes streamReader.Next() return an error.
	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()

	idleTimer := time.AfterFunc(streamIdleTimeout, cancelStream)
	defer idleTimer.Stop()

	stream, err := client.ConnectStream(streamCtx, subscription)
	if err != nil {
		return receivedEvents, err
	}

	defer stream.Close()

	handler.HandleConnectionChange(true)
	defer handler.HandleConnectionChange(false)

	streamReader := NewStreamReader(stream)

	for {
		streamEvent, err := streamReader.Next()

		if errors.Is(err, ErrInvalidEventData) {
			log.Printf("Skipped Talkdesk Live event: %v", err)

			continue
		}

		if err != nil {
			if streamCtx.Err() != nil && ctx.Err() == nil {
				return receivedEvents, fmt.Errorf("no events received for %s", streamIdleTimeout)
			}

			return receivedEvents, err
		}

		idleTimer.Reset(streamIdleTimeout)
		receivedEvents = true

		handler.HandleEvent(streamEvent)
	}
}

// removeURLFromError removes the request URL from an HTTP client error.
// Go puts the full URL in the error text, and the stream URL contains a secret token.
// Errors go into the logs, so the token must not be in them.
func removeURLFromError(err error) error {
	var urlError *url.Error

	if errors.As(err, &urlError) {
		return fmt.Errorf("%s <stream URL removed>: %w", urlError.Op, urlError.Err)
	}

	return err
}

func waitForRetry(ctx context.Context, retryDelay time.Duration) error {
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return nil
	}
}
