package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/alphagov/paas-prometheus-exporter/exporter"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	version            = "0.0.2"
	apiEndpoint        = kingpin.Flag("api-endpoint", "API endpoint").Default("https://api.10.244.0.34.xip.io").OverrideDefaultFromEnvar("API_ENDPOINT").String()
	username           = kingpin.Flag("username", "UAA username.").Default("").OverrideDefaultFromEnvar("USERNAME").String()
	password           = kingpin.Flag("password", "UAA password.").Default("").OverrideDefaultFromEnvar("PASSWORD").String()
	clientID           = kingpin.Flag("client-id", "UAA client ID.").Default("").OverrideDefaultFromEnvar("CLIENT_ID").String()
	clientSecret       = kingpin.Flag("client-secret", "UAA client secret.").Default("").OverrideDefaultFromEnvar("CLIENT_SECRET").String()
	updateFrequency    = kingpin.Flag("update-frequency", "The time in seconds, that takes between each apps update call.").Default("300").OverrideDefaultFromEnvar("UPDATE_FREQUENCY").Int64()
	prometheusBindPort = kingpin.Flag("prometheus-bind-port", "The port to bind to for prometheus metrics.").Default("8080").OverrideDefaultFromEnvar("PORT").Int()
)

func main() {
	kingpin.Parse()

	config := &cfclient.Config{
		ApiAddress:   *apiEndpoint,
		Username:     *username,
		Password:     *password,
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
	}

	cf, err := cfclient.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}

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

	exporter_cf := exporter.NewCFClient(cf)

	e := exporter.New(exporter_cf, exporter.NewWatcherManager(config))
	go e.Start(time.Duration(*updateFrequency) * time.Second)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *prometheusBindPort), nil))
}
