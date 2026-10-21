# Talkdesk Exporter

[![CI](https://github.com/tdotmich/talkdesk_exporter/actions/workflows/ci.yml/badge.svg)](https://github.com/tdotmich/talkdesk_exporter/actions/workflows/ci.yml)

A Prometheus exporter for [Talkdesk Live](https://docs.talkdesk.com/docs/live-api).
It shows your real-time contact centre metrics (for example arrived contacts,
contacts in queue, abandon rate) on a `/metrics` endpoint, so Prometheus,
Grafana or an OpenTelemetry Collector can use them.

Talkdesk does not supply a Prometheus exporter. This project fills that gap.

> This is a community project. It is not made, supported or endorsed by Talkdesk.

## How it works

The exporter subscribes to up to 16 Talkdesk Live queries. Talkdesk pushes new values
to the exporter every 5 to 60 seconds on a Server-Sent Events stream. The exporter keeps
the newest value of each query in memory. A Prometheus scrape reads these values and
never calls the Talkdesk API.

If the stream stops, the exporter connects again by itself.

More detail: [docs/how-it-works.md](docs/how-it-works.md).

## What you need

1. A Talkdesk account with API access.
2. An OAuth client (client credentials grant) with these scopes:
   - `live-queries:read`
   - `live-subscriptions:read`
   - `live-subscriptions:write`

   See the Talkdesk docs: [API access](https://docs.talkdesk.com/docs/api-access)
   and [client credentials](https://docs.talkdesk.com/docs/client-credentials).
3. The names of the Live queries you want, for example `total_arrived_contacts`.
   At start-up the exporter logs all the query names on your account.

## Install

**Binary.** Download a release for Linux, macOS or Windows from the
[releases page](https://github.com/tdotmich/talkdesk_exporter/releases).

**Docker.**

```bash
docker run -d -p 10060:10060 -e TALKDESK_CLIENT_ID -e TALKDESK_CLIENT_SECRET -v "$PWD/talkdesk_exporter.yaml:/etc/talkdesk_exporter/talkdesk_exporter.yaml:ro" ghcr.io/tdotmich/talkdesk_exporter:latest
```

**From source** (Go 1.27.3 or later).

```bash
go install github.com/tdotmich/talkdesk_exporter/cmd/talkdesk_exporter@latest
```

## Configure

`talkdesk_exporter.yaml`:

```yaml
talkdesk:
  account_name: my-account                  # your Talkdesk account name
  api_base_url: https://api.talkdeskapp.com # your region (table below)
  # token_url: https://my-account.talkdeskid.com/oauth/token   # optional
  live_queries:                             # 1 to 16 query names
    - total_arrived_contacts
```

| Region | `api_base_url` |
|---|---|
| US | `https://api.talkdeskapp.com` |
| EU | `https://api.talkdeskapp.eu` |
| Canada | `https://api.talkdeskappca.com` |
| UK | `https://api.talkdeskapp.co.uk` |

`token_url` is optional. The default is `https://<account_name>.talkdeskid.com/oauth/token`.
If your region uses a different token address, set it here.

Set the credentials as environment variables. Do not put them in the config file:

```bash
export TALKDESK_CLIENT_ID=...
export TALKDESK_CLIENT_SECRET=...
```

Run:

```bash
talkdesk_exporter -config.file talkdesk_exporter.yaml
```

| Flag | Default | Description |
|---|---|---|
| `-config.file` | `talkdesk_exporter.yaml` | Path to the config file. |
| `-web.listen-address` | `:10060` | Address for `/metrics`. |
| `-web.config.file` | (none) | Turns on TLS and basic auth. See [examples/web-config.yml](examples/web-config.yml). |
| `-version` | | Prints the version. |

## Metrics

| Metric | Type | Description |
|---|---|---|
| `talkdesk_live_query_value{query}` | gauge | Newest value of a single-number query. |
| `talkdesk_live_query_rows{query}` | gauge | Number of rows in the newest result. For a list query (for example `live_contacts_list`) this is the number of items. |
| `talkdesk_live_query_last_update_timestamp_seconds{query}` | gauge | Unix time when Talkdesk calculated the result. |
| `talkdesk_live_stream_connected` | gauge | 1 when connected to the Talkdesk stream, 0 when not. |
| `talkdesk_live_stream_events_total{query}` | counter | Result events received. |
| `talkdesk_live_stream_event_errors_total{query}` | counter | Result events the exporter could not read. |

The `query` label is the query name from your config.

A single-number query (for example `total_arrived_contacts`) has a value and a row count of 1.
A list query has only a row count. The exporter never exports the rows themselves,
because they can contain personal data such as phone numbers.

## Use with Prometheus

Add a scrape job:

```yaml
scrape_configs:
  - job_name: talkdesk
    static_configs:
      - targets: ["exporter-host:10060"]
```

Example alerts (disconnected stream, stale values) are in
[examples/alerts.yml](examples/alerts.yml). To try the exporter and Prometheus together,
use [examples/docker-compose.yml](examples/docker-compose.yml).

## Use with OpenTelemetry

Use the [Prometheus receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/prometheusreceiver)
in the OpenTelemetry Collector. It is in the `otelcol-contrib` distribution. The receiver
scrapes the exporter, and any metrics exporter (for example OTLP) sends the data on:

```yaml
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: talkdesk
          static_configs:
            - targets: ["exporter-host:10060"]

exporters:
  otlphttp:
    endpoint: https://otlp.example.com

service:
  pipelines:
    metrics:
      receivers: [prometheus]
      exporters: [otlphttp]
```

Full example: [examples/otel-collector.yaml](examples/otel-collector.yaml).

## Security

- The credentials come only from environment variables. The exporter never logs them,
  the access token, or the stream URL.
- `/metrics` contains business data. Use `-web.config.file` for TLS and basic auth,
  or keep the port on a private network.
- The Docker image is distroless and runs as a non-root user.

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## Limits

- A list query gives only its row count, not the data in the rows.
- Grouped queries (one number for each ring group, for example) are not split by group yet.
  They show as a row count. If you use one, please open an issue with an example result.
- A subscription holds no more than 16 queries (a Talkdesk limit). For more queries,
  run a second exporter.
- If Talkdesk cannot be reached at start-up, the exporter stops with an error.
  Run it under systemd, Docker or Kubernetes so it restarts.

## Development

```bash
go test ./...
```

The tests use fake Talkdesk servers. They do not need a Talkdesk account.

Issues and pull requests are welcome. This project is maintained on a best-effort basis.
If you need a change quickly, fork it. The license allows this.

## License

[MIT](LICENSE)
