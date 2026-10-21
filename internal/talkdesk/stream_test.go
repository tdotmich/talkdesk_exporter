package talkdesk

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// recordingHandler keeps what RunStream sends to it. It stops the test when the first event arrives.
type recordingHandler struct {
	events            []*StreamEvent
	connectionChanges []bool
	stop              context.CancelFunc
}

func (handler *recordingHandler) HandleEvent(streamEvent *StreamEvent) {
	handler.events = append(handler.events, streamEvent)
	handler.stop()
}

func (handler *recordingHandler) HandleConnectionChange(connected bool) {
	handler.connectionChanges = append(handler.connectionChanges, connected)
}

// TestRunStream runs the full flow against a fake Talkdesk server:
// create a subscription, connect to the stream, receive one event.
func TestRunStream(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/live-subscriptions", func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(
			writer,
			`{"id":"sub-001","_links":{"stream":{"href":"%s/live-subscriptions/sub-001/stream?token=abc"}}}`,
			server.URL,
		)
	})

	mux.HandleFunc("/live-subscriptions/sub-001/stream", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("token") != "abc" {
			t.Errorf("expected the stream token in the URL")
		}

		writer.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(writer, "event:result\n")
		fmt.Fprint(writer, `data:{"metadata":{"name":"total_arrived_contacts","timestamp":"2020-11-13T18:17:43Z"},"result":[{"_value":3}]}`+"\n\n")
		writer.(http.Flusher).Flush()

		// Keep the connection open like the real stream, until the client closes it.
		<-request.Context().Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handler := &recordingHandler{stop: cancel}

	err := newTestClient(server).RunStream(
		ctx,
		[]LiveQuery{{ID: "query-001", Name: "total_arrived_contacts"}},
		handler,
	)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if len(handler.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(handler.events))
	}

	if handler.events[0].Data.Metadata.Name != "total_arrived_contacts" {
		t.Errorf("unexpected event: %+v", handler.events[0])
	}

	// The handler must see "connected" and then "disconnected".
	if len(handler.connectionChanges) != 2 || !handler.connectionChanges[0] || handler.connectionChanges[1] {
		t.Errorf("expected connection changes [true false], got %v", handler.connectionChanges)
	}
}

// TestConnectStreamErrorHidesToken checks that the secret stream token is not in the error text.
func TestConnectStreamErrorHidesToken(t *testing.T) {
	client := &Client{httpClient: &http.Client{}}

	// Nothing listens on port 1, so the connection fails.
	subscription := &LiveSubscription{}
	subscription.Links.Stream.Href = "http://127.0.0.1:1/live-subscriptions/sub-001/stream?token=secret-token"

	_, err := client.ConnectStream(context.Background(), subscription)
	if err == nil {
		t.Fatal("expected a connection error")
	}

	if strings.Contains(err.Error(), "secret-token") {
		t.Errorf("the error contains the stream token: %v", err)
	}
}
