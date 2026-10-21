package metrics

import (
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tdotmich/talkdesk_exporter/internal/talkdesk"
)

// Metrics holds the Prometheus metrics for the exporter.
//
// Talkdesk pushes values to us on the stream. We keep the newest value of each query
// in a gauge. When Prometheus scrapes /metrics, it reads these gauges. A scrape never
// calls the Talkdesk API.
type Metrics struct {
	queryValue      *prometheus.GaugeVec
	queryRows       *prometheus.GaugeVec
	queryLastUpdate *prometheus.GaugeVec
	streamConnected prometheus.Gauge
	eventsTotal     *prometheus.CounterVec
	eventErrors     *prometheus.CounterVec

	// knownQueries holds the query names from the config. We only make series for
	// these names, so unexpected data from the stream cannot add new series.
	knownQueries map[string]bool
}

func New(registry prometheus.Registerer, queryNames []string) *Metrics {
	exporterMetrics := &Metrics{
		knownQueries: make(map[string]bool, len(queryNames)),

		queryValue: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "talkdesk_live_query_value",
				Help: "Newest value of a Talkdesk Live query.",
			},
			[]string{"query"},
		),

		// For a list query (for example a list of live contacts) the row count is the
		// useful number. We never export the rows, because they can contain personal data.
		queryRows: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "talkdesk_live_query_rows",
				Help: "Number of rows in the newest result of a Talkdesk Live query.",
			},
			[]string{"query"},
		),

		queryLastUpdate: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "talkdesk_live_query_last_update_timestamp_seconds",
				Help: "Unix time when Talkdesk calculated the newest value of the query.",
			},
			[]string{"query"},
		),

		streamConnected: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "talkdesk_live_stream_connected",
				Help: "1 when the exporter is connected to the Talkdesk Live stream, 0 when it is not.",
			},
		),

		eventsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "talkdesk_live_stream_events_total",
				Help: "Number of result events received from the Talkdesk Live stream.",
			},
			[]string{"query"},
		),

		eventErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "talkdesk_live_stream_event_errors_total",
				Help: "Number of result events whose result was not a list of rows.",
			},
			[]string{"query"},
		),
	}

	registry.MustRegister(
		exporterMetrics.queryValue,
		exporterMetrics.queryRows,
		exporterMetrics.queryLastUpdate,
		exporterMetrics.streamConnected,
		exporterMetrics.eventsTotal,
		exporterMetrics.eventErrors,
	)

	// Start the counters at 0, so they show on /metrics before the first event
	// and rate() works from the start.
	for _, queryName := range queryNames {
		exporterMetrics.knownQueries[queryName] = true
		exporterMetrics.eventsTotal.WithLabelValues(queryName)
		exporterMetrics.eventErrors.WithLabelValues(queryName)
	}

	return exporterMetrics
}

// HandleEvent updates the gauges from one stream event.
func (exporterMetrics *Metrics) HandleEvent(streamEvent *talkdesk.StreamEvent) {
	if streamEvent.Type != "result" {
		log.Printf("Ignored Talkdesk Live event: type=%q", streamEvent.Type)

		return
	}

	// We set metadata.name to the query name when we created the subscription.
	queryName := streamEvent.Data.Metadata.Name

	if !exporterMetrics.knownQueries[queryName] {
		log.Printf("Ignored Talkdesk Live result for an unknown query: %q", queryName)

		return
	}

	exporterMetrics.eventsTotal.WithLabelValues(queryName).Inc()

	rowCount, err := streamEvent.Data.RowCount()
	if err != nil {
		exporterMetrics.eventErrors.WithLabelValues(queryName).Inc()

		log.Printf("Skipped Talkdesk Live result: query=%s error=%v", queryName, err)

		return
	}

	exporterMetrics.queryRows.WithLabelValues(queryName).Set(float64(rowCount))

	// A single-number query also gets a value. A list query has no single value,
	// so it only has the row count. This is normal, not an error.
	value, err := streamEvent.Data.Value()
	if err == nil {
		exporterMetrics.queryValue.WithLabelValues(queryName).Set(value)
	}

	// If the timestamp is missing or bad, use the time we received the event.
	updatedAt, err := streamEvent.Data.Metadata.Time()
	if err != nil {
		updatedAt = time.Now()
	}

	exporterMetrics.queryLastUpdate.WithLabelValues(queryName).Set(float64(updatedAt.UnixMilli()) / 1000)
}

// HandleConnectionChange sets the connected gauge to 1 or 0.
func (exporterMetrics *Metrics) HandleConnectionChange(connected bool) {
	if connected {
		exporterMetrics.streamConnected.Set(1)

		return
	}

	exporterMetrics.streamConnected.Set(0)
}
