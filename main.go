package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alphagov/paas-prometheus-exporter/app"
	"github.com/alphagov/paas-prometheus-exporter/cf"
	"github.com/alphagov/paas-prometheus-exporter/service"
	"github.com/alphagov/paas-prometheus-exporter/util"

	"github.com/cloudfoundry-community/go-cfclient"
	cfenv "github.com/cloudfoundry-community/go-cfenv"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	version            = "0.0.5"
	apiEndpoint        = kingpin.Flag("api-endpoint", "API endpoint").Required().OverrideDefaultFromEnvar("API_ENDPOINT").String()
	logCacheEndpoint   = kingpin.Flag("logcache-endpoint", "LogCache endpoint").Default("").OverrideDefaultFromEnvar("LOGCACHE_ENDPOINT").String()
	username           = kingpin.Flag("username", "UAA username.").Default("").OverrideDefaultFromEnvar("USERNAME").String()
	password           = kingpin.Flag("password", "UAA password.").Default("").OverrideDefaultFromEnvar("PASSWORD").String()
	clientID           = kingpin.Flag("client-id", "UAA client ID.").Default("").OverrideDefaultFromEnvar("CLIENT_ID").String()
	clientSecret       = kingpin.Flag("client-secret", "UAA client secret.").Default("").OverrideDefaultFromEnvar("CLIENT_SECRET").String()
	updateFrequency    = kingpin.Flag("update-frequency", "The time in seconds, that takes between each apps update call.").Default("300").OverrideDefaultFromEnvar("UPDATE_FREQUENCY").Int64()
	scrapeInterval     = kingpin.Flag("scrape-interval", "The time in seconds, that takes between Prometheus scrapes.").Default("60").OverrideDefaultFromEnvar("SCRAPE_INTERVAL").Int64()
	prometheusBindPort = kingpin.Flag("prometheus-bind-port", "The port to bind to for prometheus metrics.").Default("8080").OverrideDefaultFromEnvar("PORT").Int()
	authUsername       = kingpin.Flag("auth-username", "HTTP basic auth username; leave blank to disable basic auth").Default("").OverrideDefaultFromEnvar("AUTH_USERNAME").String()
	authPassword       = kingpin.Flag("auth-password", "HTTP basic auth password").Default("").OverrideDefaultFromEnvar("AUTH_PASSWORD").String()
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

	if *logCacheEndpoint == "" {
		*logCacheEndpoint = strings.Replace(*apiEndpoint, "api.", "log-cache.", 1)
	}

	if *updateFrequency < 60 {
		log.Fatal("The update frequency can not be less than 1 minute")
	}

	if *scrapeInterval < 60 {
		log.Fatal("The scrape interval can not be less than 1 minute")
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

	vcapplication, err := cfenv.Current()
	if err != nil {
		log.Fatal("Could not decode the VCAP_APPLICATION environment variable")
	}

	appId := vcapplication.AppID
	appName := vcapplication.Name
	appIndex := vcapplication.Index

	// We set a unique user agent so we can
	// identify individual exporters in our logs
	userAgent := fmt.Sprintf(
		"paas-prometheus-exporter/%s (app=%s, index=%d, name=%s)",
		version,
		appId,
		appIndex,
		appName,
	)

	config := &cfclient.Config{
		ApiAddress:   *apiEndpoint,
		Username:     *username,
		Password:     *password,
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		UserAgent:    userAgent,
	}
	client, err := cf.NewClient(config, *logCacheEndpoint)
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

	serviceDiscovery := service.NewDiscovery(
		client,
		prometheus.DefaultRegisterer,
		time.Duration(*updateFrequency)*time.Second,
		time.Duration(*scrapeInterval)*time.Second,
	)

	serviceDiscovery.Start(ctx, errChan)

	server := buildHTTPServer(*prometheusBindPort, promhttp.Handler(), *authUsername, *authPassword)

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	for {
		select {

		case err := <-errChan:
			log.Println(err)
			cancel()

			defer func() {
				// This will appear as a CF app crash when the app encounters an error
				log.Println("cancel upon error finished. exiting with status code 1")
				os.Exit(1)
			}()

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

func buildHTTPServer(port int, promHandler http.Handler, authUsername, authPassword string) *http.Server {
	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promHandler)
	server.Handler = mux

	if authUsername != "" {
		server.Handler = util.BasicAuthHandler(authUsername, authPassword, "metrics", server.Handler)
	}

	return server
}
