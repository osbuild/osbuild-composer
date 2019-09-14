package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"

	"osbuild-composer/rpmmd"
	"osbuild-composer/weldr"
)

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "v", false, "Print access log")
	flag.Parse()

	err := os.Remove("/run/weldr/api.socket")
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	listener, err := net.Listen("unix", "/run/weldr/api.socket")
	if err != nil {
		panic(err)
	}

	repo := rpmmd.RepoConfig{
		Id:       "fedora-30",
		Name:     "Fedora 30",
		Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-30&arch=x86_64",
	}

	packages, err := rpmmd.FetchPackageList(repo)
	if err != nil {
		panic(err)
	}

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	api := weldr.New(repo, packages, logger)
	server := http.Server{Handler: api}

	shutdownDone := make(chan struct{}, 1)
	go func() {
		channel := make(chan os.Signal, 1)
		signal.Notify(channel, os.Interrupt)
		<-channel
		server.Shutdown(context.Background())
		close(shutdownDone)
	}()

	err = server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}

	<-shutdownDone
}
