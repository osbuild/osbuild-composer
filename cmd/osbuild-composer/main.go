package main

import (
	"flag"
	"log"
	"os"

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

	stateFile := "/var/lib/osbuild-composer/state.json"

	listeners, err := activation.Listeners()
	if err != nil {
		panic(err)
	}

	if len(listeners) != 2 {
		panic("Unexpected number of sockets. Composer require 2 of them.")
	}

	weldrListener := listeners[0]
	jobListener := listeners[1]

	repo := rpmmd.RepoConfig{
		Id:       "fedora",
		Name:     "Fedora 30",
		Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-30&arch=x86_64",
	}

	rpm := rpmmd.NewRPMMD()

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	err = os.MkdirAll("/var/lib/osbuild-composer", 0755)
	if err != nil {
		panic(err)
	}

	store := store.New(&stateFile)

	jobAPI := jobqueue.New(logger, store)
	weldrAPI := weldr.New(rpm, repo, logger, store)

	go jobAPI.Serve(jobListener)
	weldrAPI.Serve(weldrListener)
}
