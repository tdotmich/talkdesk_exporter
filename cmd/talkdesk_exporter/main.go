package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flag"
	"github.com/tdotmich/talkdesk_exporter/internal/config"
	"github.com/tdotmich/talkdesk_exporter/internal/metrics"
	"github.com/tdotmich/talkdesk_exporter/internal/talkdesk"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
)

// version is set at build time by the release pipeline (-ldflags "-X main.version=...").
var version = "dev"

func main() {

	var configFilePath string
	var listenAddress string
	var webConfigFilePath string
	var showVersion bool

	flag.StringVar(
		&configFilePath,
		"config.file",
		"talkdesk_exporter.yaml",
		"Path to the exporter configuration file",
	)

	flag.StringVar(
		&listenAddress,
		"web.listen-address",
		":10060",
		"Address for the Prometheus metrics server",
	)

	flag.StringVar(
		&webConfigFilePath,
		"web.config.file",
		"",
		"Optional path to a web config file that turns on TLS or basic auth for /metrics",
	)

	flag.BoolVar(
		&showVersion,
		"version",
		false,
		"Print the version and exit",
	)

	flag.Parse()

	if showVersion {
		fmt.Println("talkdesk_exporter", version)
		return
	}

	// ctx is cancelled when we get Ctrl+C or SIGTERM (for example from systemd or Kubernetes).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	defer stop()

	cfg, err := config.Load(configFilePath)

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting talkdesk_exporter %s", version)
	log.Printf("Talkdesk API URL: %s", cfg.Talkdesk.BaseAPIURL)
	log.Printf("Exporter listening address: %s", listenAddress)

	talkdeskClient := talkdesk.NewClient(
		ctx,
		cfg.Talkdesk.TokenURL,
		cfg.Talkdesk.ClientID,
		cfg.Talkdesk.ClientSecret,
		cfg.Talkdesk.BaseAPIURL,
	)

	// Step 1: get all the queries on the account.
	liveQueries, err := talkdeskClient.ListQueries(ctx)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Retrieved %d Talkdesk Live queries", liveQueries.Count)

	for _, liveQuery := range liveQueries.Embedded.Queries {
		log.Printf("Live query: name=%s, id=%s", liveQuery.Name, liveQuery.ID)
	}

	// Step 2: find the IDs of the queries in the config file.
	resolvedQueries, err := talkdesk.ResolveLiveQueries(
		liveQueries,
		cfg.Talkdesk.LiveQueries,
	)
	if err != nil {
		log.Fatal(err)
	}

	for _, liveQuery := range resolvedQueries {
		log.Printf(
			"Selected Live query: name=%s id=%s",
			liveQuery.Name,
			liveQuery.ID,
		)
	}

	// Step 3: start the /metrics server.
	// We use our own registry, so /metrics shows only the metrics we register here.
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	exporterMetrics := metrics.New(registry, cfg.Talkdesk.LiveQueries)

	landingPage, err := web.NewLandingPage(web.LandingConfig{
		Name:        "Talkdesk Exporter",
		Description: "Prometheus exporter for Talkdesk Live",
		Version:     version,
		Links: []web.LandingLinks{
			{Address: "/metrics", Text: "Metrics"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.Handle("/", landingPage)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// The exporter toolkit is the standard Prometheus web server. With -web.config.file
	// it adds TLS and basic auth. Without it, it serves plain HTTP.
	webFlags := &web.FlagConfig{
		WebListenAddresses: &[]string{listenAddress},
		WebSystemdSocket:   new(bool),
		WebConfigFile:      &webConfigFilePath,
	}

	go func() {
		err := web.ListenAndServe(server, webFlags, slog.Default())
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("metrics server: %v", err)
		}
	}()

	// Step 4: subscribe to the queries and update the metrics from the stream.
	// RunStream only returns when ctx is cancelled.
	err = talkdeskClient.RunStream(ctx, resolvedQueries, exporterMetrics)

	if err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}

	log.Printf("Shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("shut down metrics server: %v", err)
	}

}
