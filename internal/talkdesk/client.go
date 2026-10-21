package talkdesk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"encoding/json"

	"golang.org/x/oauth2/clientcredentials"
)

const (
	// requestTimeout applies to normal API calls. It does not apply to the stream,
	// because the stream connection stays open for hours.
	requestTimeout = 30 * time.Second

	// The Talkdesk API returns no more than 50 queries on each page.
	queriesPerPage = 50

	// maxQueryPages stops the page loop if the API ignores the page parameter.
	maxQueryPages = 100
)

type Client struct {
	httpClient *http.Client
	baseAPIURL string
}

// NewClient returns a client that gets OAuth2 tokens with the client credentials grant.
// The oauth2 package adds the token to each request and gets a new token before the old one expires.
func NewClient(
	ctx context.Context,
	tokenURL string,
	clientID string,
	clientSecret string,
	baseAPIURL string,
) *Client {

	oauthConfig := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scopes: []string{
			"live-queries:read",
			"live-subscriptions:read",
			"live-subscriptions:write",
		},
	}

	httpClient := oauthConfig.Client(ctx)

	return &Client{
		httpClient: httpClient,
		baseAPIURL: baseAPIURL,
	}
}

// ListQueries returns all the Live queries on the account. It reads each page until a page is not full.
func (client *Client) ListQueries(ctx context.Context) (*LiveQueriesResponse, error) {
	allQueries := &LiveQueriesResponse{}

	for page := 1; page <= maxQueryPages; page++ {
		queriesPage, err := client.listQueriesPage(ctx, page)
		if err != nil {
			return nil, err
		}

		allQueries.Embedded.Queries = append(allQueries.Embedded.Queries, queriesPage.Embedded.Queries...)

		if len(queriesPage.Embedded.Queries) < queriesPerPage {
			break
		}
	}

	allQueries.Count = len(allQueries.Embedded.Queries)
	allQueries.Total = len(allQueries.Embedded.Queries)

	return allQueries, nil
}

func (client *Client) listQueriesPage(ctx context.Context, page int) (*LiveQueriesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	requestURL := fmt.Sprintf(
		"%s/live-queries?page=%d&per_page=%d",
		client.baseAPIURL,
		page,
		queriesPerPage,
	)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)

	if err != nil {
		return nil, fmt.Errorf("create live queries request: %w", err)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send live queries request: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"list live queries returned HTTP status %s: %s",
			response.Status,
			readErrorBody(response),
		)
	}

	var liveQueries LiveQueriesResponse

	err = json.NewDecoder(response.Body).Decode(&liveQueries)
	if err != nil {
		return nil, fmt.Errorf("decode live queries response: %w", err)
	}

	return &liveQueries, nil
}

// readErrorBody returns the start of an error response. Talkdesk puts the reason for
// the error in the body (for example "Missing scope"), so we add it to the error message.
func readErrorBody(response *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024))
	if err != nil {
		return "(could not read response body)"
	}

	return string(body)
}
