package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ExitOk int = iota
	ExitError
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
	flag.IntVar(&argTimeoutPopulate, "timeout-populate", 300, "Timeout before the populate phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutBuild, "timeout-build", 3600, "Timeout before the build phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutExport, "timeout-export", 1800, "Timeout before the export phase needs to be completed in seconds.")

	flag.StringVar(&argBuildPath, "build-path", "/run/osbuild", "Path to use as a build directory.")

	flag.Parse()

	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if argJSON {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))
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
	Stderr  io.ReadCloser
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
		slog.Error("Builder.GuardState: Guard state mismatch", "requested", stateWanted, "current", stateCurrent)
		os.Exit(ExitError)
	}
}

func (builder *Builder) RegisterHandler(h Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			slog.Error("Handler: error", "error", err)
		}
	})
}

func (builder *Builder) HandleClaim(w http.ResponseWriter, r *http.Request) error {
	builder.GuardState(StateClaim)

	if r.Method != "POST" {
		slog.Error("Builder.HandleClaim: unexpected request method", "method", r.Method)
	}

	fmt.Fprintf(w, "%s", "done")

	slog.Info("Builder.HandleClaim: Done")

	builder.SetState(StateProvision)

	return nil
}

func (builder *Builder) HandleProvision(w http.ResponseWriter, r *http.Request) (err error) {
	builder.GuardState(StateProvision)

	if r.Method != "PUT" {
		return fmt.Errorf("Builder.HandleProvision: Unexpected request method")
	}

	slog.Debug("Builder.HandleProvision: Opening manifest.json", "argBuildPath", argBuildPath)

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

	slog.Debug("Builder.HandleProvision: Writing manifest.json")

	_, err = io.Copy(dst, r.Body)

	if err != nil {
		return fmt.Errorf("Builder.HandleProvision: Failed to write manifest.json")
	}

	w.WriteHeader(http.StatusCreated)

	if _, err := w.Write([]byte(`done`)); err != nil {
		return fmt.Errorf("Builder.HandleProvision: Failed to write response")
	}

	slog.Info("Builder.HandleProvision: Done")

	builder.SetState(StatePopulate)

	return nil
}

func (builder *Builder) HandlePopulate(w http.ResponseWriter, r *http.Request) error {
	builder.GuardState(StatePopulate)

	if r.Method != "POST" {
		return fmt.Errorf("Builder.HandlePopulate: unexpected request method")
	}
	storePath := path.Join(argBuildPath, "store")
	err := os.Mkdir(storePath, 0755)
	if err != nil {
		return fmt.Errorf("Builder.HandlePopulate: failed to make store directory: %v", err)
	}

	tarReader := tar.NewReader(r.Body)
	for header, err := tarReader.Next(); err != io.EOF; header, err = tarReader.Next() {
		if err != nil {
			return fmt.Errorf("Builder.HandlerPopulate: failed to unpack sources: %v", err)
		}

		// gosec seems overly zealous here, as the destination gets verified
		dest := filepath.Join(storePath, header.Name) // #nosec G305
		if !strings.HasPrefix(dest, filepath.Clean(storePath)) {
			return fmt.Errorf("Builder.HandlerPopulate: name not clean: %v doesn't have %v prefix", dest, filepath.Clean(storePath))
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(dest, header.FileInfo().Mode()); err != nil {
				return fmt.Errorf("Builder.HandlerPopulate: unable to make dir in sources: %v", err)
			}
		case tar.TypeReg:
			file, err := os.Create(dest)
			if err != nil {
				return fmt.Errorf("Builder.HandlerPopulate: unable to open file in sources: %v", err)
			}
			defer file.Close()

			// the inputs are trusted so ignore G110
			_, err = io.Copy(file, tarReader) // #nosec G110
			if err != nil {
				return fmt.Errorf("Builder.HandlerPopulate: unable to write file in sources: %v", err)
			}
			file.Close()
		default:
			return fmt.Errorf("Builder.HandlerPopulate: unexpected tar header type: %v", header.Typeflag)
		}
	}

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(`done`)); err != nil {
		return fmt.Errorf("Builder.HandlePopulate: Failed to write response")
	}

	slog.Info("Builder.HandlePopulate: Done")

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
		slog.Error("HandleBuild: Build started but Build was non-nil")
		os.Exit(ExitError)
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

	slog.Info("BackgroundProcess: Starting", "process", builder.Build.Process, "env", envs)

	builder.Build.Stdout = &bytes.Buffer{}
	builder.Build.Process.Stdout = builder.Build.Stdout

	builder.Build.Stderr, err = builder.Build.Process.StderrPipe()

	if err != nil {
		return err
	}

	if err := builder.Build.Process.Start(); err != nil {
		return fmt.Errorf("BackgroundProcess: Failed to start process")
	}

	go func() {
		builder.Build.Error = builder.Build.Process.Wait()
		builder.Build.Done = true

		slog.Info("BackgroundProcess: Exited")
	}()

	go func() {
		scanner := bufio.NewScanner(builder.Build.Stderr)
		for scanner.Scan() {
			m := scanner.Text()
			slog.Info("BackgroundProcess: Stderr", "text", m)
		}
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
		w.WriteHeader(http.StatusOK)

		if builder.Build.Error != nil {
			return fmt.Errorf("Builder.HandleBuild: Buildprocess exited with error: %s", builder.Build.Error)
		}

		if _, err := w.Write(builder.Build.Stdout.Bytes()); err != nil {
			return fmt.Errorf("Builder.HandleBuild: Failed to write response")
		}

		builder.SetState(StateExport)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}

	slog.Info("Builder.HandleBuild: Done")
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

	slog.Info("Builder.HandleExport: Done")

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

	/* #nosec G112 */
	builder.net = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", builder.Host, builder.Port),
		Handler: mux,
	}

	return builder.net.ListenAndServe()
}

func main() {
	slog.With(
		slog.Bool("argJSON", argJSON),
		slog.String("argBuilderHost", argBuilderHost),
		slog.Int("argBuilderPort", argBuilderPort),
		slog.Int("argTimeoutClaim", argTimeoutClaim),
		slog.Int("argTimeoutProvision", argTimeoutProvision),
		slog.Int("argTimeoutBuild", argTimeoutBuild),
		slog.Int("argTimeoutExport", argTimeoutExport),
	).Info("main: Starting up")

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
					slog.Error("main: server shutdown failed", "err", err)
				}
				cancel()
				slog.Info("main: Shutting down successfully")
				os.Exit(ExitOk)
			}
		case err := <-errs:
			slog.Error("ErrorChannel", "err", err)
			os.Exit(ExitError)
		}
	}
}
