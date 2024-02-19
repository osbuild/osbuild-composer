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

func (builder *Builder) GuardState(stateWanted State) error {
	if stateCurrent := builder.GetState(); stateWanted != stateCurrent {
		return fmt.Errorf("Builder.GuardState: Requested guard for %d but we're in %d. Exit", stateWanted, stateCurrent)
	}

	return nil
}

func (builder *Builder) RegisterHandler(s State, m string, h Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := builder.GuardState(s); err != nil {
			w.WriteHeader(http.StatusConflict)
			logrus.Fatalf("Received request in invalid state: %d. Exiting", s)
		}

		if r.Method != m {
			w.WriteHeader(http.StatusBadRequest)
			logrus.Fatalf("Received request with invalid method: %s while expecting %s in state %d. Exiting", r.Method, m, s)
		}

		if err := h(w, r); err != nil {
			logrus.Fatalf("Error executing handler in state: %d, error: %v", s, err)
		}
	})
}

func (b *Builder) HandleClaim(w http.ResponseWriter, r *http.Request) error {
	logrus.Info("Builder.HandleClaim: Done")

	w.WriteHeader(http.StatusOK)
	b.SetState(StateProvision)

	return nil
}

func (b *Builder) HandleProvision(w http.ResponseWriter, r *http.Request) error {
	logrus.WithFields(logrus.Fields{"argBuildPath": argBuildPath}).Debug("Builder.HandleProvision: Opening manifest.json")

	dst, err := os.Create(
		path.Join(argBuildPath, "manifest.json"),
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("Builder.HandleProvision: Failed to open manifest.json: %v", err)
	}

	logrus.Debug("Builder.HandleProvision: Writing manifest.json")

	_, err = io.Copy(dst, r.Body)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("Builder.HandleProvision: Failed to write manifest.json: %v", err)
	}

	if err = dst.Close(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("Builder.HandleProvision: Failed to close manifest.json: %v", err)
	}

	logrus.Info("Builder.HandleProvision: Done")

	w.WriteHeader(http.StatusCreated)
	b.SetState(StatePopulate)

	return nil
}

func (b *Builder) HandlePopulate(w http.ResponseWriter, r *http.Request) error {
	logrus.Info("Builder.HandlePopulate: Done")

	w.WriteHeader(http.StatusOK)
	b.SetState(StateBuild)

	return nil
}

func (b *Builder) HandleBuild(w http.ResponseWriter, r *http.Request) error {
	var buildRequest BuildRequest

	if err := json.NewDecoder(r.Body).Decode(&buildRequest); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("HandleBuild: Failed to decode body: %v", err)
	}

	if b.Build != nil {
		w.WriteHeader(http.StatusInternalServerError)
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

	b.Build = &BackgroundProcess{}
	b.Build.Process = exec.Command(
		"/usr/bin/osbuild",
		args...,
	)
	b.Build.Process.Env = envs

	logrus.Infof("BackgroundProcess: Starting %v with %s", b.Build.Process, envs)

	b.Build.Stdout = &bytes.Buffer{}
	b.Build.Stderr = &bytes.Buffer{}

	b.Build.Process.Stdout = b.Build.Stdout
	b.Build.Process.Stderr = b.Build.Stderr

	if err := b.Build.Process.Start(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("BackgroundProcess: Failed to start process")
	}

	go func() {
		b.Build.Error = b.Build.Process.Wait()
		b.Build.Done = true

		logrus.Info("BackgroundProcess: Exited")
	}()

	w.WriteHeader(http.StatusCreated)
	b.SetState(StateProgress)

	return nil
}

func (b *Builder) HandleProgress(w http.ResponseWriter, r *http.Request) error {
	if b.Build == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("HandleProgress: Progress requested but Build was nil")
	}

	if !b.Build.Done {
		logrus.Info("Builder.HandleBuild: In Progress")
		w.WriteHeader(http.StatusAccepted)
		return nil
	}

	// If there was an error in the build process we respond with an appropriate error status code and include
	// stderr in the body of the response before exiting.
	if b.Build.Error != nil {
		w.WriteHeader(http.StatusConflict)

		if _, err := w.Write(b.Build.Stderr.Bytes()); err != nil {
			return fmt.Errorf("Builder.HandleBuild: Failed to write stderr response: %v", err)
		}

		return fmt.Errorf("Builder.HandleBuild: Buildprocess exited with error: %v", b.Build.Error)
	}

	// Otherwise we respond with OK and include stdout instead.
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(b.Build.Stdout.Bytes()); err != nil {
		return fmt.Errorf("Builder.HandleBuild: Failed to write stdout response: %v", err)
	}

	b.SetState(StateExport)

	logrus.Info("Builder.HandleBuild: Done")

	return nil
}

func (b *Builder) HandleExport(w http.ResponseWriter, r *http.Request) error {
	exportPath := r.URL.Query().Get("path")

	if exportPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("Builder.HandleExport: Missing export")
	}

	// XXX check subdir
	srcPath := path.Join(argBuildPath, "export", exportPath)

	src, err := os.Open(
		srcPath,
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("Builder.HandleExport: Failed to open source: %v", err)
	}

	_, err = io.Copy(w, src)

	if err != nil {
		return fmt.Errorf("Builder.HandleExport: Failed to write response: %v", err)
	}

	logrus.Info("Builder.HandleExport: Done")

	b.SetState(StateDone)

	return nil
}

func (builder *Builder) Serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/claim", builder.RegisterHandler(StateClaim, "POST", builder.HandleClaim))

	mux.HandleFunc("/provision", builder.RegisterHandler(StateProvision, "PUT", builder.HandleProvision))
	mux.HandleFunc("/populate", builder.RegisterHandler(StatePopulate, "POST", builder.HandlePopulate))

	mux.HandleFunc("/build", builder.RegisterHandler(StateBuild, "POST", builder.HandleBuild))
	mux.HandleFunc("/progress", builder.RegisterHandler(StateProgress, "GET", builder.HandleProgress))

	mux.HandleFunc("/export", builder.RegisterHandler(StateExport, "GET", builder.HandleExport))

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
