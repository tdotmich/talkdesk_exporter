package talkdesk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient returns a client for a fake Talkdesk server. It does not use OAuth2.
func newTestClient(server *httptest.Server) *Client {
	return &Client{
		httpClient: server.Client(),
		baseAPIURL: server.URL,
	}
}

func TestListQueriesReadsAllPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/live-queries" {
			t.Errorf("unexpected path %q", request.URL.Path)
		}

		// Page 1 is full (50 queries). Page 2 has 1 query, so it is the last page.
		queryCount := 50
		if request.URL.Query().Get("page") == "2" {
			queryCount = 1
		}

		var response LiveQueriesResponse
		for i := 0; i < queryCount; i++ {
			response.Embedded.Queries = append(response.Embedded.Queries, LiveQuery{
				ID:   fmt.Sprintf("id-%s-%d", request.URL.Query().Get("page"), i),
				Name: fmt.Sprintf("query_%s_%d", request.URL.Query().Get("page"), i),
			})
		}

		_ = json.NewEncoder(writer).Encode(response)
	}))
	defer server.Close()

	liveQueries, err := newTestClient(server).ListQueries(context.Background())
	if err != nil {
		t.Fatalf("ListQueries returned an unexpected error: %v", err)
	}

	if liveQueries.Count != 51 {
		t.Errorf("expected 51 queries, got %d", liveQueries.Count)
	}
}

func TestListQueriesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, `{"code":"0700002","message":"Missing scope"}`, http.StatusForbidden)
	}))
	defer server.Close()

	_, err := newTestClient(server).ListQueries(context.Background())
	if err == nil {
		t.Fatal("expected an error for HTTP 403")
	}
}

func TestCreateSubscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/live-subscriptions" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}

		var subscriptionRequest CreateSubscriptionRequest

		err := json.NewDecoder(request.Body).Decode(&subscriptionRequest)
		if err != nil {
			t.Errorf("decode request body: %v", err)
		}

		if len(subscriptionRequest.Queries) != 1 ||
			subscriptionRequest.Queries[0].ID != "query-001" ||
			subscriptionRequest.Queries[0].Metadata.Name != "total_arrived_contacts" {
			t.Errorf("unexpected request body: %+v", subscriptionRequest)
		}

		_, _ = writer.Write([]byte(`{
			"id": "sub-001",
			"_links": {"stream": {"href": "https://example.com/live-subscriptions/sub-001/stream?token=abc"}}
		}`))
	}))
	defer server.Close()

	subscription, err := newTestClient(server).CreateSubscription(
		context.Background(),
		[]LiveQuery{{ID: "query-001", Name: "total_arrived_contacts"}},
	)
	if err != nil {
		t.Fatalf("CreateSubscription returned an unexpected error: %v", err)
	}

	if subscription.ID != "sub-001" {
		t.Errorf("expected subscription ID %q, got %q", "sub-001", subscription.ID)
	}

	if subscription.Links.Stream.Href == "" {
		t.Error("expected a stream URL")
	}
}
