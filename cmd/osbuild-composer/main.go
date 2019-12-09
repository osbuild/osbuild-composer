package main

import (
	"flag"
	"log"
	"os"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"

	"github.com/coreos/go-systemd/activation"
)

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "v", false, "Print access log")
	flag.Parse()

	stateDir := "/var/lib/osbuild-composer"

	listeners, err := activation.Listeners()
	if err != nil {
		panic(err)
	}

	if len(listeners) != 2 {
		panic("Unexpected number of sockets. Composer require 2 of them.")
	}

	weldrListener := listeners[0]
	jobListener := listeners[1]

	rpm := rpmmd.NewRPMMD()

	distribution, err := distro.FromHost()
	if err != nil {
		panic("cannot detect distro from host: " + err.Error())
	}

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	store := store.New(&stateDir, distribution)

	jobAPI := jobqueue.New(logger, store)
	weldrAPI := weldr.New(rpm, distribution, logger, store)

	go jobAPI.Serve(jobListener)
	weldrAPI.Serve(weldrListener)
}
