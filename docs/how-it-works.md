# How it works

## Data flow

Talkdesk Live is **push**. Talkdesk sends new values on an open HTTP connection
(Server-Sent Events, SSE). Prometheus is **pull**. It asks `/metrics` for values every
scrape interval. The exporter connects the two:

```
                 1. GET  /live-queries          (find query IDs by name)
                 2. POST /live-subscriptions    (subscribe to the IDs)
 Talkdesk  <---- 3. GET  <stream URL>           (SSE stream stays open)
    |
    |  event:result
    |  data:{"metadata":{"name":"total_arrived_contacts",...},"result":[{"_value":12}]}
    v
 Exporter  ----> keeps the newest value of each query in a gauge
    ^
    |  GET /metrics (every scrape)
 Prometheus
```

A scrape never calls the Talkdesk API. It only reads the gauges in memory. This keeps
scrapes fast, and the Talkdesk API rate limits do not depend on the scrape interval.

## Code layout

| File | What it does |
|---|---|
| `cmd/talkdesk_exporter/main.go` | Start-up. Reads flags and config, finds the queries, starts the `/metrics` server, then runs the stream. |
| `internal/config/config.go` | Reads the YAML file and the two secrets from environment variables. Checks the values. |
| `internal/talkdesk/client.go` | The HTTP client. OAuth2 (client credentials) and the list of queries, with paging. |
| `internal/talkdesk/queries.go` | Finds the ID of each query name in the config. Stops on an unknown or duplicate name. |
| `internal/talkdesk/subscriptions.go` | Creates a Live subscription for the queries. |
| `internal/talkdesk/stream.go` | Connects to the stream, and connects again when it stops (with a wait that doubles). |
| `internal/talkdesk/events.go` | Reads SSE events from the stream and gets the number out of each result. |
| `internal/metrics/metrics.go` | The Prometheus metrics. Updates the gauges from each event. |

## Design decisions

- **Gauges, not a custom collector.** Talkdesk pushes values to us, so we store each value
  when it arrives. A custom collector is for exporters that call an API at scrape time.
- **Own registry.** `prometheus.NewRegistry()` shows only the metrics we register.
  The default global registry can contain metrics from other packages.
- **Only configured query names become series.** Unexpected data from the stream cannot
  add new label values, so the number of series stays fixed.
- **New subscription for each connection.** The stream URL has a token in it, and the token
  can expire. A new subscription always gives a new token, so a reconnect always works.
- **Wait that doubles (exponential backoff).** After a failure we wait 1s, then 2s, 4s,
  up to 30s. This stops us from flooding Talkdesk when it is down. After a good connection
  the wait goes back to 1s.
- **Idle timeout.** A network connection can die with no error. Talkdesk sends data at least
  every 60 seconds. If nothing arrives for 2 minutes, we close the connection and connect again.
- **One bad event does not stop the stream.** A bad event is logged, counted and skipped.
- **Secrets in environment variables.** They do not go into a file or into Git. The config
  decoder rejects unknown fields, so a secret in the file is an error.
- **No secrets in logs.** Go puts the request URL in HTTP errors. The stream URL contains
  a token, so `removeURLFromError` removes the URL before the error is logged.
- **`StreamHandler` interface.** The `talkdesk` package does not know about Prometheus.
  It sends events to any type with `HandleEvent` and `HandleConnectionChange`. This keeps
  the packages separate and lets the tests use a fake handler.
- **Exporter toolkit for the web server.** It is the standard Prometheus web server. It gives
  TLS and basic auth through `-web.config.file`, the same as `node_exporter`.
- **Graceful shutdown.** Ctrl+C or SIGTERM cancels the context. The stream stops, and the
  HTTP server finishes open requests before it closes.

## Port

The default port is 10060. Ports 9100 to 9999 in the
[Prometheus port list](https://github.com/prometheus/prometheus/wiki/Default-port-allocations)
are all taken, and 10060 was the next free number.
