package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	ExitOk int = iota
)

type State int

type Handler func(w http.ResponseWriter, r *http.Request) error

const (
	StateClaim State = iota
	StateProvision
	StatePopulate
	StateBuild
	StateProgress
	StateExport
	StateDone

	StateError
	StateSignal
	StateTimeout
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

type BuildRequest struct {
	Pipelines    []string `json:"pipelines"`
	Environments []string `json:"environments"`
}

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

type Builder struct {
	Host         string
	Port         int
	State        State
	StateLock    sync.Mutex
	StateChannel chan State
	Build        *BackgroundProcess

	net *http.Server
}

type BackgroundProcess struct {
	Process *exec.Cmd
	Stdout  *bytes.Buffer
	Stderr  *bytes.Buffer
	Done    bool
	Error   error
}

func (builder *Builder) SetState(state State) {
	builder.StateLock.Lock()
	defer builder.StateLock.Unlock()

	if state <= builder.State {
		builder.State = StateError
	} else {
		builder.State = state
	}

	builder.StateChannel <- builder.State
}

func (builder *Builder) GetState() State {
	builder.StateLock.Lock()
	defer builder.StateLock.Unlock()

	return builder.State
}

func (builder *Builder) GuardState(stateWanted State) {
	if stateCurrent := builder.GetState(); stateWanted != stateCurrent {
		logrus.Fatalf("Builder.GuardState: Requested guard for %d but we're in %d. Exit", stateWanted, stateCurrent)
	}
}

func (builder *Builder) RegisterHandler(h Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			logrus.Fatal(err)
		}
	})
}

func (builder *Builder) HandleClaim(w http.ResponseWriter, r *http.Request) error {
	builder.GuardState(StateClaim)

	if r.Method != "POST" {
		logrus.WithFields(
			logrus.Fields{"method": r.Method},
		).Fatal("Builder.HandleClaim: unexpected request method")
	}

	fmt.Fprintf(w, "%s", "done")

	logrus.Info("Builder.HandleClaim: Done")

	builder.SetState(StateProvision)

	return nil
}

func (builder *Builder) HandleProvision(w http.ResponseWriter, r *http.Request) (err error) {
	builder.GuardState(StateProvision)

	if r.Method != "PUT" {
		return fmt.Errorf("Builder.HandleProvision: Unexpected request method")
	}

	logrus.WithFields(logrus.Fields{"argBuildPath": argBuildPath}).Debug("Builder.HandleProvision: Opening manifest.json")

	dst, err := os.OpenFile(
		path.Join(argBuildPath, "manifest.json"),
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0400,
	)

	defer func() {
		if cerr := dst.Close(); cerr != nil {
			err = cerr
		}
	}()

	if err != nil {
		return fmt.Errorf("Builder.HandleProvision: Failed to open manifest.json")
	}

	logrus.Debug("Builder.HandleProvision: Writing manifest.json")

	_, err = io.Copy(dst, r.Body)

	if err != nil {
		return fmt.Errorf("Builder.HandleProvision: Failed to write manifest.json")
	}

	w.WriteHeader(http.StatusCreated)

	if _, err := w.Write([]byte(`done`)); err != nil {
		return fmt.Errorf("Builder.HandleProvision: Failed to write response")
	}

	logrus.Info("Builder.HandleProvision: Done")

	builder.SetState(StatePopulate)

	return nil
}

func (builder *Builder) HandlePopulate(w http.ResponseWriter, r *http.Request) error {
	builder.GuardState(StatePopulate)

	if r.Method != "POST" {
		return fmt.Errorf("Builder.HandlePopulate: unexpected request method")
	}

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(`done`)); err != nil {
		return fmt.Errorf("Builder.HandlePopulate: Failed to write response")
	}

	logrus.Info("Builder.HandlePopulate: Done")

	builder.SetState(StateBuild)

	return nil
}

func (builder *Builder) HandleBuild(w http.ResponseWriter, r *http.Request) error {
	builder.GuardState(StateBuild)

	if r.Method != "POST" {
		return fmt.Errorf("Builder.HandleBuild: Unexpected request method")
	}

	var buildRequest BuildRequest

	var err error

	if err = json.NewDecoder(r.Body).Decode(&buildRequest); err != nil {
		return fmt.Errorf("HandleBuild: Failed to decode body")
	}

	if builder.Build != nil {
		logrus.Fatal("HandleBuild: Build started but Build was non-nil")
	}

	args := []string{
		"--store", path.Join(argBuildPath, "store"),
		"--cache-max-size", "0",
		"--output-directory", path.Join(argBuildPath, "export"),
		"--json",
	}

	for _, pipeline := range buildRequest.Pipelines {
		args = append(args, "--export")
		args = append(args, pipeline)
	}

	args = append(args, path.Join(argBuildPath, "manifest.json"))

	envs := os.Environ()
	envs = append(envs, buildRequest.Environments...)

	builder.Build = &BackgroundProcess{}
	builder.Build.Process = exec.Command(
		"/usr/bin/osbuild",
		args...,
	)
	builder.Build.Process.Env = envs

	logrus.Infof("BackgroundProcess: Starting %s with %s", builder.Build.Process, envs)

	builder.Build.Stdout = &bytes.Buffer{}
	builder.Build.Process.Stdout = builder.Build.Stdout
	builder.Build.Process.Stderr = builder.Build.Stderr

	if err != nil {
		return err
	}

	if err := builder.Build.Process.Start(); err != nil {
		return fmt.Errorf("BackgroundProcess: Failed to start process")
	}

	go func() {
		builder.Build.Error = builder.Build.Process.Wait()
		builder.Build.Done = true

		logrus.Info("BackgroundProcess: Exited")
	}()

	w.WriteHeader(http.StatusCreated)

	builder.SetState(StateProgress)

	return nil
}

func (builder *Builder) HandleProgress(w http.ResponseWriter, r *http.Request) error {
	builder.GuardState(StateProgress)

	if r.Method != "GET" {
		return fmt.Errorf("Builder.HandleProgress: Unexpected request method")
	}

	if builder.Build == nil {
		return fmt.Errorf("HandleProgress: Progress requested but Build was nil")
	}

	if builder.Build.Done {
		if builder.Build.Error != nil {
			w.WriteHeader(http.StatusConflict)

			if _, err := w.Write(builder.Build.Stderr.Bytes()); err != nil {
				return fmt.Errorf("Builder.HandleBuild: Failed to write stderr response")
			}

			return fmt.Errorf("Builder.HandleBuild: Buildprocess exited with error: %s", builder.Build.Error)
		}

		w.WriteHeader(http.StatusOK)

		if _, err := w.Write(builder.Build.Stdout.Bytes()); err != nil {
			return fmt.Errorf("Builder.HandleBuild: Failed to write stdout response")
		}

		builder.SetState(StateExport)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}

	logrus.Info("Builder.HandleBuild: Done")
	return nil
}

func (builder *Builder) HandleExport(w http.ResponseWriter, r *http.Request) error {
	builder.GuardState(StateExport)

	if r.Method != "GET" {
		return fmt.Errorf("Builder.HandleExport: unexpected request method")
	}

	exportPath := r.URL.Query().Get("path")

	if exportPath == "" {
		return fmt.Errorf("Builder.HandleExport: Missing export")
	}

	// XXX check subdir
	srcPath := path.Join(argBuildPath, "export", exportPath)

	src, err := os.Open(
		srcPath,
	)

	if err != nil {
		return fmt.Errorf("Builder.HandleExport: Failed to open source: %s", err)
	}

	_, err = io.Copy(w, src)

	if err != nil {
		return fmt.Errorf("Builder.HandleExport: Failed to write response: %s", err)
	}

	logrus.Info("Builder.HandleExport: Done")

	builder.SetState(StateDone)

	return nil
}

func (builder *Builder) Serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/claim", builder.RegisterHandler(builder.HandleClaim))

	mux.HandleFunc("/provision", builder.RegisterHandler(builder.HandleProvision))
	mux.HandleFunc("/populate", builder.RegisterHandler(builder.HandlePopulate))

	mux.HandleFunc("/build", builder.RegisterHandler(builder.HandleBuild))
	mux.HandleFunc("/progress", builder.RegisterHandler(builder.HandleProgress))

	mux.HandleFunc("/export", builder.RegisterHandler(builder.HandleExport))

	builder.net = &http.Server{
		ReadTimeout:       1 * time.Second,
		WriteTimeout:      1800 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		Addr:              fmt.Sprintf("%s:%d", builder.Host, builder.Port),
		Handler:           mux,
	}

	return builder.net.ListenAndServe()
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

	builder := Builder{
		State:        StateClaim,
		StateChannel: make(chan State, 1),
		Host:         argBuilderHost,
		Port:         argBuilderPort,
	}

	errs := make(chan error, 1)

	go func(errs chan<- error) {
		if err := builder.Serve(); err != nil {
			errs <- err
		}
	}(errs)

	for {
		select {
		case state := <-builder.StateChannel:
			if state == StateDone {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := builder.net.Shutdown(ctx); err != nil {
					logrus.Errorf("main: server shutdown failed: %v", err)
				}
				cancel()
				logrus.Info("main: Shutting down successfully")
				os.Exit(ExitOk)
			}
		case err := <-errs:
			logrus.Fatal(err)
		}
	}
}
