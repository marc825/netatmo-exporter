package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exzz/netatmo-api-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"

	"github.com/marc825/netatmo-exporter/v2/internal/collector"
	"github.com/marc825/netatmo-exporter/v2/internal/config"
	"github.com/marc825/netatmo-exporter/v2/internal/logger"
	"github.com/marc825/netatmo-exporter/v2/internal/token"
	"github.com/marc825/netatmo-exporter/v2/internal/web"
)

var (
	signals = []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
	}

	log = logger.NewLogger()
)

func main() {
	cfg, err := config.Parse(os.Args, os.Getenv)
	switch {
	case err == pflag.ErrHelp:
		return
	case err != nil:
		log.Fatalf("Error in configuration: %s", err)
	default:
	}

	log.SetLevel(logrus.Level(cfg.LogLevel))
	log.Infof("netatmo-exporter %s (commit: %s)", Version, GitCommit)

	// Netatmo API client
	client := netatmo.NewClient(cfg.Netatmo, tokenUpdated(cfg.TokenFile))

	// Load token from file if available
	if cfg.TokenFile != "" {
		token, err := loadToken(cfg.TokenFile)
		switch {
		case os.IsNotExist(err):
			// no token file yet
		case err != nil:
			log.Fatalf("Error loading token: %s", err)
		case !token.Expiry.IsZero() && token.Expiry.Before(time.Now()):
			log.Warn("Restored token has expired! Token has been ignored.")
		default:
			if token.RefreshToken == "" {
				log.Warn("Restored token has no refresh-token! Exporter will need to be re-authenticated manually.")
			} else if token.Expiry.IsZero() {
				log.Warn("Restored token has no expiry time! Token will be renewed immediately.")
				token.Expiry = time.Now().Add(time.Second)
			}

			log.Infof("Loaded token from %s.", cfg.TokenFile)
			client.InitWithToken(context.Background(), token)
		}

		registerSignalHandler(client, cfg.TokenFile)
	} else {
		log.Warn("No token-file set! Authentication will be lost on restart.")
	}

	// Prometheus registryV1 V1 separate for Weather + HomeCoach
	registryV1 := prometheus.NewRegistry()
	// V2 unified registry combining Weather + HomeCoach
	registryV2 := prometheus.NewRegistry()

	var weatherReader collector.WeatherReadFunction
	var homecoachReader collector.HomecoachReadFunction

	// Weather station collector V1
	if cfg.EnableWeather {
		// Weather reader function for unified collector V2
		weatherReader = client.Read

		// Weather reader function V1
		weatherMetrics := collector.NewWeatherReadFunction(log, weatherReader, cfg.RefreshInterval, cfg.StaleDuration)
		registryV1.MustRegister(weatherMetrics)
	} else {
		log.Info("Weather station collector disabled by configuration.")
	}

	if cfg.EnableHomecoach {
		// Homecoach reader function V1 + V2 Definition
		homecoachReader = collector.NewHomecoachReadFunction(client.CurrentToken)

		// Homecoach reader function V1
		homecoachMetrics := collector.NewHomecoachCollector(log, homecoachReader, cfg.RefreshInterval, cfg.StaleDuration)
		registryV1.MustRegister(homecoachMetrics)
	} else {
		log.Info("HomeCoach collector disabled by configuration.")
	}

	// Token metrics for V1 + V2
	tokenMetric := token.Metric(client.CurrentToken)
	registryV1.MustRegister(tokenMetric)
	registryV2.MustRegister(tokenMetric)

	// Unified collector V2 for Weather + HomeCoach
	unifiedCollector := collector.UnifiedCollector(
		log,
		weatherReader,
		homecoachReader,
		cfg.RefreshInterval,
		cfg.StaleDuration,
		cfg.EnableWeather,
		cfg.EnableHomecoach,
	)
	registryV2.MustRegister(unifiedCollector)

	if cfg.EnableGoMetrics {
		log.Info("Go runtime metrics enabled.")
		registryV1.MustRegister(prometheus.NewGoCollector())
		registryV1.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
		registryV2.MustRegister(prometheus.NewGoCollector())
		registryV2.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	} else {
		log.Info("Go runtime metrics disabled.")
	}

	if cfg.DebugHandlers {
		// Combined debug handler for Weather + HomeCoach
		http.Handle("/debug/netatmo", web.DebugNetatmoHandler(log, weatherReader, homecoachReader))
		http.Handle("/debug/token", web.DebugTokenHandler(log, client.CurrentToken))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	http.Handle("/auth/authorize", web.AuthorizeHandler(cfg.ExternalURL, client))
	http.Handle("/auth/callback", web.CallbackHandler(ctx, client))
	http.Handle("/auth/settoken", web.SetTokenHandler(ctx, client))

	http.Handle("/metrics/v1", promhttp.HandlerFor(registryV1, promhttp.HandlerOpts{}))
	http.Handle("/metrics/v2", promhttp.HandlerFor(registryV2, promhttp.HandlerOpts{}))
	http.Handle("/version", versionHandler(log))
	http.Handle("/", web.HomeHandler(client.CurrentToken))

	log.Infof("Listen on %s...", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, nil))
}

func loadToken(fileName string) (*oauth2.Token, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

func registerSignalHandler(client *netatmo.Client, fileName string) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)

	go func() {
		sig := <-ch
		signal.Reset(signals...)

		log.Debugf("Got signal: %s", sig)

		if err := saveToken(client, fileName); err != nil {
			log.Errorf("Error persisting token: %s", err)
		}

		os.Exit(0)
	}()
}

func tokenUpdated(fileName string) netatmo.TokenUpdateFunc {
	if fileName == "" {
		return nil
	}

	return func(token *oauth2.Token) {
		log.Debugf("Token updated. Expires: %s", token.Expiry)

		if err := saveTokenFile(fileName, token); err != nil {
			log.Errorf("Error saving token: %s", err)
		}
	}
}

func saveToken(client *netatmo.Client, fileName string) error {
	token, err := client.CurrentToken()
	switch {
	case err == netatmo.ErrNotAuthenticated:
		return nil
	case err != nil:
		return fmt.Errorf("error retrieving token: %w", err)
	default:
	}

	log.Infof("Saving token to %s ...", fileName)

	return saveTokenFile(fileName, token)
}

func saveTokenFile(fileName string, token *oauth2.Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("error marshalling token: %w", err)
	}

	if err := os.WriteFile(fileName, data, 0o600); err != nil {
		return fmt.Errorf("error writing token file: %w", err)
	}

	return nil
}
