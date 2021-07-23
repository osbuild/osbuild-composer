package main

import (
	"flag"
	"log"
	"os"

	"github.com/coreos/go-systemd/activation"
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
	flag.BoolVar(&verbose, "v", false, "Print access log")
	flag.Parse()

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			config = &ComposerConfigFile{}
		} else {
			log.Fatalf("Error loading configuration: %v", err)
		}
	}

	log.Println("Loaded configuration:")
	err = DumpConfig(config, log.Writer())
	if err != nil {
		log.Fatalf("Error printing configuration: %v", err)
	}

	stateDir, ok := os.LookupEnv("STATE_DIRECTORY")
	if !ok {
		log.Fatal("STATE_DIRECTORY is not set. Is the service file missing StateDirectory=?")
	}

	cacheDir, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		log.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}

	composer, err := NewComposer(config, stateDir, cacheDir, logger)
	if err != nil {
		log.Fatalf("%v", err)
	}

	listeners, err := activation.ListenersWithNames()
	if err != nil {
		log.Fatalf("Could not get listening sockets: " + err.Error())
	}

	if l, exists := listeners["osbuild-composer.socket"]; exists {
		if len(l) != 1 {
			log.Fatal("The osbuild-composer.socket unit is misconfigured. It should contain only one socket.")
		}

		err = composer.InitWeldr(repositoryConfigs, l[0])
		if err != nil {
			log.Fatalf("Error initializing weldr API: %v", err)
		}
	}

	if l, exists := listeners["osbuild-local-worker.socket"]; exists {
		if len(l) != 1 {
			log.Fatal("The osbuild-local-worker.socket unit is misconfigured. It should contain only one socket.")
		}

		composer.InitLocalWorker(l[0])
	}

	if l, exists := listeners["osbuild-composer-api.socket"]; exists {
		if len(l) != 1 {
			log.Fatal("The osbuild-composer-api.socket unit is misconfigured. It should contain only one socket.")
		}

		err = composer.InitAPI(ServerCertFile, ServerKeyFile, config.EnableJWT, l[0])
		if err != nil {
			log.Fatalf("Error initializing koji API: %v", err)
		}
	}

	if l, exists := listeners["osbuild-remote-worker.socket"]; exists {
		if len(l) != 1 {
			log.Fatal("The osbuild-remote-worker.socket unit is misconfigured. It should contain only one socket.")
		}

		err = composer.InitRemoteWorkers(ServerCertFile, ServerKeyFile, l[0])
		if err != nil {
			log.Fatalf("Error initializing worker API: %v", err)
		}
	}

	err = composer.Start()
	if err != nil {
		log.Fatalf("%v", err)
	}
}
