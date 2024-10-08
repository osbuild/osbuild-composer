package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"

	"github.com/coreos/go-systemd/activation"
	"github.com/coreos/go-systemd/journal"
	"github.com/getsentry/sentry-go"
	sentrylogrus "github.com/getsentry/sentry-go/logrus"
	"github.com/osbuild/osbuild-composer/internal/common"
	slogger "github.com/osbuild/osbuild-composer/pkg/splunk_logger"
	"github.com/sirupsen/logrus"
)

const (
	configFile     = "/etc/osbuild-composer/osbuild-composer.toml"
	ServerKeyFile  = "/etc/osbuild-composer/composer-key.pem"
	ServerCertFile = "/etc/osbuild-composer/composer-crt.pem"
)

var repositoryConfigs = []string{
	"/etc/osbuild-composer",
	"/usr/share/osbuild-composer",
}

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "Print access log")
	flag.Parse()

	// Redirect Go standard logger into logrus before it's used by other packages
	log.SetOutput(common.Logger())
	// Ensure the Go standard logger does not have any prefix or timestamp
	log.SetFlags(0)

	if !verbose {
		logrus.Print("verbose flag is provided for backward compatibility only, current behavior is always printing the access log")
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		logrus.Fatalf("Error loading configuration: %v", err)
	}

	logrus.SetOutput(os.Stdout)
	logLevel, err := logrus.ParseLevel(config.LogLevel)

	logrus.SetReportCaller(true)
	// Add context hook to log operation_id and external_id
	logrus.AddHook(&common.ContextHook{})

	if err == nil {
		logrus.SetLevel(logLevel)
	} else {
		logrus.Info("Failed to load loglevel from config:", err)
	}

	switch config.LogFormat {
	case "journal":
		// If we are running under systemd, use the journal. Otherwise,
		// fallback to text formatter.
		if journal.Enabled() {
			logrus.SetFormatter(&logrus.JSONFormatter{})
			logrus.AddHook(&common.JournalHook{})
			logrus.SetOutput(io.Discard)
		} else {
			logrus.SetFormatter(&logrus.TextFormatter{})
		}
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		logrus.Infof("Failed to set logging format from config, '%s' is not a valid option", config.LogFormat)
	}

	logrus.Info("Loaded configuration:")
	err = DumpConfig(*config, logrus.StandardLogger().WriterLevel(logrus.InfoLevel))
	if err != nil {
		logrus.Fatalf("Error printing configuration: %v", err)
	}

	if config.DeploymentChannel != "" {
		logrus.AddHook(&slogger.EnvironmentHook{Channel: config.DeploymentChannel})
	}

	if config.SplunkHost != "" {
		hook, err := slogger.NewSplunkHook(context.Background(), config.SplunkHost, config.SplunkPort, config.SplunkToken, "osbuild-composer")

		if err != nil {
			panic(err)
		}
		logrus.AddHook(hook)
	}

	if config.GlitchTipDSN != "" {
		err = sentry.Init(sentry.ClientOptions{
			Dsn: config.GlitchTipDSN,
		})
		if err != nil {
			panic(err)
		}

		sentryhook := sentrylogrus.NewFromClient([]logrus.Level{logrus.PanicLevel,
			logrus.FatalLevel, logrus.ErrorLevel},
			sentry.CurrentHub().Client())
		logrus.AddHook(sentryhook)

	} else {
		logrus.Warn("GLITCHTIP_DSN not configured, skipping initializing Sentry/Glitchtip")
	}

	stateDir, ok := os.LookupEnv("STATE_DIRECTORY")
	if !ok {
		logrus.Fatal("STATE_DIRECTORY is not set. Is the service file missing StateDirectory=?")
	}

	cacheDir, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		logrus.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}

	composer, err := NewComposer(config, stateDir, cacheDir)
	if err != nil {
		logrus.Fatalf("%v", err)
	}

	listeners, err := activation.ListenersWithNames()
	if err != nil {
		logrus.Fatalf("Could not get listening sockets: %s", err.Error())
	}

	if l, exists := listeners["osbuild-composer.socket"]; exists {
		if len(l) != 2 {
			logrus.Fatal("The osbuild-composer.socket unit is misconfigured. It should contain two sockets.")
		}

		err = composer.InitWeldr(l[0], config.weldrDistrosImageTypeDenyList())
		if err != nil {
			logrus.Fatalf("Error initializing weldr API: %v", err)
		}

		// Start cloudapi using the 2nd socket and no certs
		err = composer.InitAPI(ServerCertFile, ServerKeyFile, false, false, false, l[1])
		if err != nil {
			logrus.Fatalf("Error initializing Cloud API using local socket: %v", err)
		}
	}

	if l, exists := listeners["osbuild-local-worker.socket"]; exists {
		if len(l) != 1 {
			logrus.Fatal("The osbuild-local-worker.socket unit is misconfigured. It should contain only one socket.")
		}

		composer.InitLocalWorker(l[0])
	}

	if l, exists := listeners["osbuild-composer-prometheus.socket"]; exists {
		if len(l) != 1 {
			logrus.Warn("The osbuild-composer-prometheus.socket unit is misconfigured. It should contain only one socket.")
		}

		composer.InitMetricsAPI(l[0])
	}

	if l, exists := listeners["osbuild-composer-api.socket"]; exists {
		if len(l) != 1 {
			logrus.Fatal("The osbuild-composer-api.socket unit is misconfigured. It should contain only one socket.")
		}

		err = composer.InitAPI(ServerCertFile, ServerKeyFile, config.Koji.EnableTLS, config.Koji.EnableMTLS, config.Koji.EnableJWT, l[0])
		if err != nil {
			logrus.Fatalf("Error initializing koji API: %v", err)
		}
	}

	if l, exists := listeners["osbuild-remote-worker.socket"]; exists {
		if len(l) != 1 {
			logrus.Fatal("The osbuild-remote-worker.socket unit is misconfigured. It should contain only one socket.")
		}

		err = composer.InitRemoteWorkers(ServerCertFile, ServerKeyFile, config.Worker.EnableTLS, config.Worker.EnableMTLS, config.Worker.EnableJWT, l[0])
		if err != nil {
			logrus.Fatalf("Error initializing worker API: %v", err)
		}
	}

	err = composer.Start()
	if err != nil {
		logrus.Fatalf("%v", err)
	}
}
