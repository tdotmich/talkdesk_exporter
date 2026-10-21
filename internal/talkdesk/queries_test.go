package talkdesk

import "testing"

func TestResolveLiveQueries(t *testing.T) {
	availableQueries := &LiveQueriesResponse{
		Embedded: LiveQueriesEmbedded{
			Queries: []LiveQuery{
				{
					ID:   "query-001",
					Name: "total_arrived_contacts",
				},
				{
					ID:   "query-002",
					Name: "contacts_in_queue",
				},
			},
		},
	}

	requestedQueryNames := []string{
		"total_arrived_contacts",
	}

	resolvedQueries, err := ResolveLiveQueries(
		availableQueries,
		requestedQueryNames,
	)
	if err != nil {
		t.Fatalf("ResolveLiveQueries returned an unexpected error: %v", err)
	}

	if len(resolvedQueries) != 1 {
		t.Fatalf(
			"expected 1 resolved query, got %d",
			len(resolvedQueries),
		)
	}

	if resolvedQueries[0].ID != "query-001" {
		t.Errorf(
			"expected query ID %q, got %q",
			"query-001",
			resolvedQueries[0].ID,
		)
	}
}

func TestResolveLiveQueriesUnknownQuery(t *testing.T) {
	availableQueries := &LiveQueriesResponse{
		Embedded: LiveQueriesEmbedded{
			Queries: []LiveQuery{
				{
					ID:   "query-001",
					Name: "total_arrived_contacts",
				},
			},
		},
	}

	requestedQueryNames := []string{
		"does_not_exist",
	}

	_, err := ResolveLiveQueries(
		availableQueries,
		requestedQueryNames,
	)

	if err == nil {
		t.Fatal("expected an error for an unknown query")
	}
}

func TestResolveLiveQueriesDuplicateQuery(t *testing.T) {
	availableQueries := &LiveQueriesResponse{
		Embedded: LiveQueriesEmbedded{
			Queries: []LiveQuery{
				{
					ID:   "query-001",
					Name: "total_arrived_contacts",
				},
			},
		},
	}

	requestedQueryNames := []string{
		"total_arrived_contacts",
		"total_arrived_contacts",
	}

	_, err := ResolveLiveQueries(
		availableQueries,
		requestedQueryNames,
	)

	if err == nil {
		t.Fatal("expected an error for a duplicate query")
	}
}
