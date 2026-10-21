package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tdotmich/talkdesk_exporter/internal/talkdesk"
)

func newResultEvent(queryName string, result string) *talkdesk.StreamEvent {
	return &talkdesk.StreamEvent{
		Type: "result",
		Data: talkdesk.StreamEventData{
			Metadata: talkdesk.StreamEventMetaData{
				Name:      queryName,
				Timestamp: "2020-11-13T18:17:43.5Z",
			},
			Result: []byte(result),
		},
	}
}

func TestHandleEventSetsGauges(t *testing.T) {
	exporterMetrics := New(prometheus.NewRegistry(), []string{"total_arrived_contacts", "live_contacts_list"})

	exporterMetrics.HandleEvent(newResultEvent("total_arrived_contacts", `[{"_value":42}]`))

	value := testutil.ToFloat64(exporterMetrics.queryValue.WithLabelValues("total_arrived_contacts"))
	if value != 42 {
		t.Errorf("expected value 42, got %v", value)
	}

	lastUpdate := testutil.ToFloat64(exporterMetrics.queryLastUpdate.WithLabelValues("total_arrived_contacts"))
	if lastUpdate != 1605291463.5 {
		t.Errorf("expected timestamp 1605291463.5, got %v", lastUpdate)
	}

	events := testutil.ToFloat64(exporterMetrics.eventsTotal.WithLabelValues("total_arrived_contacts"))
	if events != 1 {
		t.Errorf("expected 1 event, got %v", events)
	}
}

func TestHandleEventCountsBadResults(t *testing.T) {
	exporterMetrics := New(prometheus.NewRegistry(), []string{"total_arrived_contacts", "live_contacts_list"})

	exporterMetrics.HandleEvent(newResultEvent("live_contacts_list", `[]`))

	errorCount := testutil.ToFloat64(exporterMetrics.eventErrors.WithLabelValues("live_contacts_list"))
	if errorCount != 1 {
		t.Errorf("expected 1 event error, got %v", errorCount)
	}

	// A bad result must not create a value for the query.
	if testutil.CollectAndCount(exporterMetrics.queryValue) != 0 {
		t.Error("expected no talkdesk_live_query_value series")
	}
}

func TestHandleConnectionChange(t *testing.T) {
	exporterMetrics := New(prometheus.NewRegistry(), []string{"total_arrived_contacts", "live_contacts_list"})

	exporterMetrics.HandleConnectionChange(true)

	if testutil.ToFloat64(exporterMetrics.streamConnected) != 1 {
		t.Error("expected talkdesk_live_stream_connected to be 1")
	}

	exporterMetrics.HandleConnectionChange(false)

	if testutil.ToFloat64(exporterMetrics.streamConnected) != 0 {
		t.Error("expected talkdesk_live_stream_connected to be 0")
	}
}

func TestHandleEventIgnoresUnknownQueries(t *testing.T) {
	exporterMetrics := New(prometheus.NewRegistry(), []string{"total_arrived_contacts"})

	exporterMetrics.HandleEvent(newResultEvent("not_in_the_config", `[{"_value":1}]`))

	if testutil.CollectAndCount(exporterMetrics.queryValue) != 0 {
		t.Error("expected no series for a query that is not in the config")
	}
}
