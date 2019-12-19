package main

import (
	"flag"
	"log"
	"os"
	"runtime"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"

	"github.com/coreos/go-systemd/activation"
)

func currentArch() string {
	if runtime.GOARCH == "amd64" {
		return "x86_64"
	} else if runtime.GOARCH == "arm64" {
		return "aarch64"
	} else if runtime.GOARCH == "ppc64le" {
		return "ppc64le"
	} else if runtime.GOARCH == "s390x" {
		return "s390x"
	} else {
		panic("unsupported architecture")
	}
}

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "v", false, "Print access log")
	flag.Parse()

	stateDir := "/var/lib/osbuild-composer"

	listeners, err := activation.Listeners()
	if err != nil {
		log.Fatalf("Could not get listening sockets: " + err.Error())
	}

	if len(listeners) != 2 {
		log.Fatalf("Unexpected number of listening sockets (%d), expected 2", len(listeners))
	}

	weldrListener := listeners[0]
	jobListener := listeners[1]

	rpm := rpmmd.NewRPMMD()
	distros := distro.NewRegistry()

	distribution, err := distros.FromHost()
	if err != nil {
		log.Fatalf("Could not determine distro from host: " + err.Error())
	}

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	store := store.New(&stateDir, distribution)

	jobAPI := jobqueue.New(logger, store)
	weldrAPI := weldr.New(rpm, currentArch(), distribution, logger, store)

	go jobAPI.Serve(jobListener)
	weldrAPI.Serve(weldrListener)
}
