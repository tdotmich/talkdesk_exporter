package talkdesk

type LiveQueriesResponse struct {
	Total    int                 `json:"total"`
	Count    int                 `json:"count"`
	Page     int                 `json:"page"`
	PerPage  int                 `json:"per_page"`
	Embedded LiveQueriesEmbedded `json:"_embedded"`
}

type LiveQueriesEmbedded struct {
	Queries []LiveQuery `json:"queries"`
}

type LiveQuery struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
