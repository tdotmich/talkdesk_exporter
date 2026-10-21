package talkdesk

import "fmt"

func ResolveLiveQueries(availableQueries *LiveQueriesResponse, requestedQueryNames []string) ([]LiveQuery, error) {
	queryByName := make(map[string]LiveQuery, len(availableQueries.Embedded.Queries))

	for _, liveQuery := range availableQueries.Embedded.Queries {
		queryByName[liveQuery.Name] = liveQuery
	}

	resolvedQueries := make([]LiveQuery, 0, len(requestedQueryNames))

	requestNameSeen := make(map[string]bool, len(requestedQueryNames))

	for _, queryName := range requestedQueryNames {
		if requestNameSeen[queryName] {
			return nil, fmt.Errorf("live query %q has been configured more than once", queryName)
		}

		requestNameSeen[queryName] = true

		liveQuery, exists := queryByName[queryName]

		if !exists {
			return nil, fmt.Errorf("configured live query %q was not returned by Talkdesk", queryName)
		}

		resolvedQueries = append(resolvedQueries, liveQuery)
	}

	return resolvedQueries, nil

}
