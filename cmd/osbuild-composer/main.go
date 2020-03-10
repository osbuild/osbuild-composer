// +build !debug

package main

import (
	"crypto/tls"
	"flag"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/rcm"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"
	"log"
	"os"

	"github.com/coreos/go-systemd/activation"
)

func main() {
	// Parse command line arguments
	var verbose bool
	flag.BoolVar(&verbose, "v", false, "Print access log")
	flag.Parse()

	// Set up sockets
	listeners, err := activation.ListenersWithNames()
	if err != nil {
		log.Fatalf("Could not get listening sockets: " + err.Error())
	}

	if _, exists := listeners["osbuild-composer.socket"]; !exists {
		log.Fatalf("osbuild-composer.socket doesn't exist")
	}

	composerListeners := listeners["osbuild-composer.socket"]

	if len(composerListeners) != 2 && len(composerListeners) != 3 {
		log.Fatalf("Unexpected number of listening sockets (%d), expected 2 or 3", len(composerListeners))
	}
	weldrListener := composerListeners[0]
	jobListener := composerListeners[1]

	// Set up distribution
	rpm, distribution, distros := createDistroConfiguration([]string{"/etc/osbuild-composer", "/usr/share/osbuild-composer"})

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	stateDir := "/var/lib/osbuild-composer"
	store := store.New(&stateDir, distribution, *distros)

	jobAPI := jobqueue.New(logger, store)
	weldrAPI := weldr.New(rpm, common.CurrentArch(), distribution, logger, store)

	go func() {
		err := jobAPI.Serve(jobListener)
		common.PanicOnError(err)
	}()

	// Optionally run RCM API as well as Weldr API
	if rcmApiListeners, exists := listeners["osbuild-rcm.socket"]; exists {
		if len(rcmApiListeners) != 1 {
			// Use Fatal to call os.Exit with non-zero return value
			log.Fatal("The RCM API socket unit is misconfigured. It should contain only one socket.")
		}
		rcmListener := rcmApiListeners[0]
		rcmAPI := rcm.New(logger, store, rpm)
		go func() {
			err := rcmAPI.Serve(rcmListener)
			// If the RCM API fails, take down the whole process, not just a single gorutine
			log.Fatal("RCM API failed: ", err)
		}()

	}

	if remoteWorkerListeners, exists := listeners["osbuild-remote-worker.socket"]; exists {
		for _, listener := range remoteWorkerListeners {
			log.Printf("Starting remote listener\n")

			tlsConfig, err := createTLSConfig(&connectionConfig{
				CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
				ServerKeyFile:  "/etc/osbuild-composer/composer-key.pem",
				ServerCertFile: "/etc/osbuild-composer/composer-crt.pem",
			})

			if err != nil {
				log.Fatalf("TLS configuration cannot be created: " + err.Error())
			}

			listener := tls.NewListener(listener, tlsConfig)
			go func() {
				err := jobAPI.Serve(listener)
				common.PanicOnError(err)
			}()
		}
	}

	err = weldrAPI.Serve(weldrListener)
	common.PanicOnError(err)

}
