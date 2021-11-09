package main

import (
	"flag"
	"os"

	"github.com/coreos/go-systemd/activation"
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

	if err == nil {
		logrus.SetLevel(logLevel)
	} else {
		logrus.Info("Failed to load loglevel from config:", err)
	}

	switch config.LogFormat {
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
		logrus.Fatalf("Could not get listening sockets: " + err.Error())
	}

	if l, exists := listeners["osbuild-composer.socket"]; exists {
		if len(l) != 1 {
			logrus.Fatal("The osbuild-composer.socket unit is misconfigured. It should contain only one socket.")
		}

		err = composer.InitWeldr(repositoryConfigs, l[0], config.weldrDistrosImageTypeDenyList())
		if err != nil {
			logrus.Fatalf("Error initializing weldr API: %v", err)
		}
	}

	if l, exists := listeners["osbuild-local-worker.socket"]; exists {
		if len(l) != 1 {
			logrus.Fatal("The osbuild-local-worker.socket unit is misconfigured. It should contain only one socket.")
		}

		composer.InitLocalWorker(l[0])
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
