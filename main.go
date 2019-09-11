package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"

	"osbuild-composer/rpmmd"
	"osbuild-composer/weldr"
)

type Composer struct{}

func (c *Composer) Repositories() []string {
	return []string{"fedora-30"}
}

func (c *Composer) RepositoryConfig(name string) (rpmmd.RepoConfig, bool) {
	if name != "fedora-30" {
		return rpmmd.RepoConfig{}, false
	}

	return rpmmd.RepoConfig{
		Id:       "fedora-30",
		Name:     "Fedora 30",
		Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-30&arch=x86_64",
	}, true
}

func main() {
	listener, err := net.Listen("unix", "/run/weldr/api.socket")
	if err != nil {
		panic(err)
	}

	composer := Composer{}
	api := weldr.New(&composer)
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
