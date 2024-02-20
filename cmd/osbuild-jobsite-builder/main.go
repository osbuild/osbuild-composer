package main

import (
	"flag"
	"os"

	"github.com/osbuild/osbuild-composer/pkg/jobsite"
	"github.com/osbuild/osbuild-composer/pkg/jobsite/builder"
	"github.com/sirupsen/logrus"
)

var (
	argJSON bool

	argBuilderHost string
	argBuilderPort int

	argTimeoutClaim     int
	argTimeoutProvision int
	argTimeoutPopulate  int
	argTimeoutBuild     int
	argTimeoutExport    int

	argBuildPath string
)

func init() {
	flag.BoolVar(&argJSON, "json", false, "Enable JSON output")

	flag.StringVar(&argBuilderHost, "builder-host", "localhost", "Hostname or IP where this program will listen on.")
	flag.IntVar(&argBuilderPort, "builder-port", 3333, "Port this program will listen on.")

	flag.IntVar(&argTimeoutClaim, "timeout-claim", 600, "Timeout before the claim phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutProvision, "timeout-provision", 30, "Timeout before the provision phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutPopulate, "timeout-populate", 30, "Timeout before the populate phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutBuild, "timeout-build", 3600, "Timeout before the build phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutExport, "timeout-export", 1800, "Timeout before the export phase needs to be completed in seconds.")

	flag.StringVar(&argBuildPath, "build-path", "/run/osbuild", "Path to use as a build directory.")

	flag.Parse()

	logrus.SetLevel(logrus.InfoLevel)

	if argJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
}

func main() {
	logrus.WithFields(
		logrus.Fields{
			"argJSON":             argJSON,
			"argBuilderHost":      argBuilderHost,
			"argBuilderPort":      argBuilderPort,
			"argTimeoutClaim":     argTimeoutClaim,
			"argTimeoutProvision": argTimeoutProvision,
			"argTimeoutBuild":     argTimeoutBuild,
			"argTimeoutExport":    argTimeoutExport,
		}).Info("main: Starting up")

	b := builder.Builder{
		State:        builder.StateClaim,
		StateChannel: make(chan builder.State, 1),
		Host:         argBuilderHost,
		Port:         argBuilderPort,
		BuildPath:    argBuildPath,
	}

	errs := make(chan error, 1)

	go func(errs chan<- error) {
		if err := b.Serve(); err != nil {
			errs <- err
		}
	}(errs)

	for {
		select {
		case state := <-b.StateChannel:
			if state == builder.StateDone {
				logrus.Info("main: Shutting down successfully")
				os.Exit(jobsite.ExitOk)
			}
		case err := <-errs:
			logrus.Fatal(err)
		}
	}
}
