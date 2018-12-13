package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alphagov/paas-prometheus-exporter/app"
	"github.com/alphagov/paas-prometheus-exporter/cf"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	version            = "0.0.3"
	apiEndpoint        = kingpin.Flag("api-endpoint", "API endpoint").Default("https://api.10.244.0.34.xip.io").OverrideDefaultFromEnvar("API_ENDPOINT").String()
	username           = kingpin.Flag("username", "UAA username.").Default("").OverrideDefaultFromEnvar("USERNAME").String()
	password           = kingpin.Flag("password", "UAA password.").Default("").OverrideDefaultFromEnvar("PASSWORD").String()
	clientID           = kingpin.Flag("client-id", "UAA client ID.").Default("").OverrideDefaultFromEnvar("CLIENT_ID").String()
	clientSecret       = kingpin.Flag("client-secret", "UAA client secret.").Default("").OverrideDefaultFromEnvar("CLIENT_SECRET").String()
	updateFrequency    = kingpin.Flag("update-frequency", "The time in seconds, that takes between each apps update call.").Default("300").OverrideDefaultFromEnvar("UPDATE_FREQUENCY").Int64()
	prometheusBindPort = kingpin.Flag("prometheus-bind-port", "The port to bind to for prometheus metrics.").Default("8080").OverrideDefaultFromEnvar("PORT").Int()
)

type ServiceDiscovery interface {
	Start()
	Stop()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Reset(syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		cancel()
	}()

	kingpin.Parse()

	build_info := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "paas_exporter_build_info",
			Help: "PaaS Prometheus exporter build info.",
			ConstLabels: prometheus.Labels{
				"version": version,
			},
		},
	)
	build_info.Set(1)
	prometheus.DefaultRegisterer.MustRegister(build_info)

	config := &cfclient.Config{
		ApiAddress:   *apiEndpoint,
		Username:     *username,
		Password:     *password,
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
	}
	client, err := cf.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}

	errChan := make(chan error, 1)

	appDiscovery := app.NewDiscovery(
		client,
		prometheus.DefaultRegisterer,
		time.Duration(*updateFrequency)*time.Second,
	)

	appDiscovery.Start(ctx, errChan)

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", *prometheusBindPort),
	}
	http.Handle("/metrics", promhttp.Handler())

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	for {
		select {
		case <-errChan:
			log.Println(err)
			cancel()
		case <-ctx.Done():
			log.Println("exiting")
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
			defer shutdownCancel()

			err := server.Shutdown(shutdownCtx)
			if err != nil {
				log.Println(err)
			}
			return
		}
	}
}
