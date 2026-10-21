package talkdesk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type CreateSubscriptionRequest struct {
	Queries []SubscriptionQuery `json:"queries"`
}

type SubscriptionQuery struct {
	ID       string               `json:"id"`
	Metadata SubscriptionMetadata `json:"metadata"`
	Params   map[string]any       `json:"params"`
}

// SubscriptionMetadata is returned to us in every stream event for the query.
// We put the query name here, so each event tells us which query it belongs to.
type SubscriptionMetadata struct {
	Name string `json:"name"`
}

type LiveSubscription struct {
	ID    string            `json:"id"`
	Links SubscriptionLinks `json:"_links"`
}

type SubscriptionLinks struct {
	Stream SubscriptionLink `json:"stream"`
	Self   SubscriptionLink `json:"self"`
}

type SubscriptionLink struct {
	Href string `json:"href"`
}

// Executes after ResolveLiveQueries
func (client *Client) CreateSubscription(ctx context.Context, resolvedQueries []LiveQuery) (*LiveSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	subscriptionRequest := CreateSubscriptionRequest{
		Queries: make([]SubscriptionQuery, 0, len(resolvedQueries)),
	}

	for _, resolvedQuery := range resolvedQueries {
		subscriptionRequest.Queries = append(subscriptionRequest.Queries, SubscriptionQuery{
			ID: resolvedQuery.ID,
			Metadata: SubscriptionMetadata{
				Name: resolvedQuery.Name,
			},
			Params: map[string]any{},
		})
	}

	requestBody, err := json.Marshal(subscriptionRequest)
	if err != nil {
		return nil, fmt.Errorf("encode subscription request: %w", err)
	}

	requestURL := fmt.Sprintf("%s/live-subscriptions", client.baseAPIURL)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create subscription HTTP request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send subscription request: %w", err)
	}

	defer response.Body.Close()

	// The Talkdesk docs say 200. We also accept 201, the usual status for "created".
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(
			"create subscription returned HTTP status %s: %s",
			response.Status,
			readErrorBody(response),
		)
	}

	var subscription LiveSubscription

	decoder := json.NewDecoder(response.Body)

	err = decoder.Decode(&subscription)
	if err != nil {
		return nil, fmt.Errorf("decode subscription response: %w", err)
	}

	return &subscription, nil
}
