// # `jobsite-manager`
package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	ExitOk int = iota
	ExitError
	ExitTimeout
	ExitSignal
)

type ArgumentList []string

func (AL *ArgumentList) String() string {
	return ""
}

func (AL *ArgumentList) Set(value string) error {
	*AL = append(*AL, value)
	return nil
}

var (
	argJSON bool

	argJobsiteHost string
	argJobsitePort int
	argBuilderHost string
	argBuilderPort int

	argTimeoutClaim     int
	argTimeoutProvision int
	argTimeoutPopulate  int
	argTimeoutBuild     int
	argTimeoutProgress  int
	argTimeoutExport    int

	argPipelines    ArgumentList
	argEnvironments ArgumentList
	argExports      ArgumentList
	argOutputPath   string
	argStore        string
)

type BuildRequest struct {
	Pipelines    []string `json:"pipelines"`
	Environments []string `json:"environments"`
}

type Step func(chan<- struct{}, chan<- error)

func init() {
	flag.BoolVar(&argJSON, "json", false, "Enable JSON output")

	flag.StringVar(&argJobsiteHost, "manager-host", "localhost", "Hostname or IP where this program will listen on.")
	flag.IntVar(&argJobsitePort, "manager-port", 3333, "Port this program will listen on.")

	flag.StringVar(&argBuilderHost, "builder-host", "localhost", "Hostname or IP of a jobsite-builder that this program will connect to.")
	flag.IntVar(&argBuilderPort, "builder-port", 3333, "Port of a jobsite-builder that this program will connect to.")

	flag.IntVar(&argTimeoutClaim, "timeout-claim", 600, "Timeout before the claim phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutProvision, "timeout-provision", 30, "Timeout before the provision phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutPopulate, "timeout-populate", 300, "Timeout before the populate phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutBuild, "timeout-build", 30, "Timeout before the build phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutProgress, "timeout-progress", 3600, "Timeout before the progress phase needs to be completed in seconds.")
	flag.IntVar(&argTimeoutExport, "timeout-export", 1800, "Timeout before the export phase needs to be completed in seconds.")

	flag.Var(&argPipelines, "export", "Pipelines to export. Can be passed multiple times.")
	flag.Var(&argExports, "export-file", "Files to export. Can be passed multiple times.")

	flag.Var(&argEnvironments, "environment", "Environments to add. Can be passed multiple times.")
	flag.StringVar(&argOutputPath, "output", "/dev/null", "Output directory to write to.")
	flag.StringVar(&argStore, "store", "", "Store to send to builder.")

	flag.Parse()

	logrus.SetLevel(logrus.InfoLevel)

	if argJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
}

func main() {
	logrus.Info("main: Starting up")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{}, 1)
	errs := make(chan error, 1)

	go Dance(done, errs)

	for {
		select {
		case sig := <-sigs:
			logrus.WithFields(
				logrus.Fields{
					"signal": sig,
				}).Info("main: Exiting on signal")
			os.Exit(ExitSignal)
		case err := <-errs:
			logrus.WithFields(
				logrus.Fields{
					"error": err,
				}).Info("main: Exiting on error")
			os.Exit(ExitError)
		case <-done:
			logrus.Info("main: Shutting down succesfully")
			os.Exit(ExitOk)
		}
	}
}

func Dance(done chan<- struct{}, errs chan<- error) {
	manifest, err := io.ReadAll(os.Stdin)

	if err != nil {
		errs <- err
		return
	}

	if err := StepClaim(); err != nil {
		errs <- err
		return
	}

	if err := StepProvision(manifest); err != nil {
		errs <- err
		return
	}

	if err := StepPopulate(); err != nil {
		errs <- err
		return
	}

	if err := StepBuild(); err != nil {
		errs <- err
		return
	}

	if err := StepProgress(); err != nil {
		errs <- err
		return
	}

	if err := StepExport(); err != nil {
		errs <- err
		return
	}

	close(done)
}

func Request(method string, path string, body io.Reader) (*http.Response, error) {
	cli := &http.Client{}
	url := fmt.Sprintf("http://%s:%d/%s", argBuilderHost, argBuilderPort, path)

	req, err := http.NewRequest(method, url, body)

	if err != nil {
		return nil, err
	}

	// Don't accept any compression on the return value; it intermittently leads to unexpected EOFs during the larger
	// download when the exports are requested.
	req.Header.Set("Accept-Encoding", "identity")

	logrus.Debugf("Request: Making a %s request to %s", method, url)

	for {
		res, err := cli.Do(req)

		if err != nil {
			if errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
				time.Sleep(1 * time.Second)
				continue
			}

			return nil, err
		}

		return res, nil
	}
}

func Wait(timeout int, fn Step) error {
	done := make(chan struct{}, 1)
	errs := make(chan error, 1)

	go fn(done, errs)

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		return fmt.Errorf("timeout")
	case <-done:
		return nil
	case err := <-errs:
		return err
	}
}

func StepClaim() error {
	return Wait(argTimeoutClaim, func(done chan<- struct{}, errs chan<- error) {
		res, err := Request("POST", "claim", nil)

		if err != nil {
			errs <- err
			return
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			errs <- fmt.Errorf("StepClaim: Got an unexpected response %d while expecting %d. Exiting", res.StatusCode, http.StatusOK)
			return
		}

		logrus.Info("StepClaim: Done")

		close(done)
	})
}

func StepProvision(manifest []byte) error {
	return Wait(argTimeoutProvision, func(done chan<- struct{}, errs chan<- error) {
		res, err := Request("PUT", "provision", bytes.NewBuffer(manifest))

		if err != nil {
			errs <- err
			return
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusCreated {
			errs <- fmt.Errorf("StepProvision: Got an unexpected response %d while expecting %d. Exiting", res.StatusCode, http.StatusCreated)
			return
		}

		logrus.Info("StepProvision: Done")

		close(done)
	})
}

func StepPopulate() error {
	return Wait(argTimeoutPopulate, func(done chan<- struct{}, errs chan<- error) {
		file, err := os.CreateTemp(filepath.Dir(argStore), "store.*.tar")
		if err != nil {
			errs <- err
			return
		}
		tw := tar.NewWriter(file)
		defer tw.Close()
		err = filepath.Walk(argStore, func(filePath string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// skip the store direction itself, just package the contents
			if filePath == argStore {
				return nil
			}

			if !fi.Mode().IsRegular() && !fi.Mode().IsDir() {
				return nil
			}
			header, err := tar.FileInfoHeader(fi, fi.Name())
			if err != nil {
				return err
			}

			// FileInfo only contains the basename, see https://pkg.go.dev/archive/tar#FileInfoHeader.
			header.Name = strings.TrimPrefix(strings.Replace(filePath, argStore, "", -1), string(filepath.Separator))
			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if fi.Mode().IsRegular() {
				f, err := os.Open(filePath)
				if err != nil {
					return err
				}
				defer f.Close()
				if err != nil {
					return err
				}
				if _, err := io.Copy(tw, f); err != nil {
					return err
				}
				f.Close()
			}
			return nil
		})
		if err != nil {
			errs <- err
			return
		}

		err = tw.Close()
		if err != nil {
			errs <- err
			return
		}

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			errs <- err
			return
		}

		res, err := Request("POST", "populate", file)
		if err != nil {
			errs <- err
			return
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			errs <- fmt.Errorf("StepPopulate: Got an unexpected response %d while expecting %d. Exiting", res.StatusCode, http.StatusOK)
			return
		}

		logrus.Info("StepPopulate: Done")

		close(done)
	})
}

func StepBuild() error {
	return Wait(argTimeoutBuild, func(done chan<- struct{}, errs chan<- error) {
		arg := BuildRequest{
			Pipelines:    argPipelines,
			Environments: argEnvironments,
		}

		dat, err := json.Marshal(arg)

		if err != nil {
			logrus.Fatalf("StepBuild: Failed to marshal data: %s", err)
		}

		res, err := Request("POST", "build", bytes.NewBuffer(dat))

		if err != nil {
			errs <- err
			return
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusCreated {
			errs <- fmt.Errorf("StepBuild: Got an unexpected response %d while expecting %d. Exiting", res.StatusCode, http.StatusOK)
			return
		}

		logrus.Info("StepBuild: Done")

		close(done)
	})
}

func StepProgress() error {
	return Wait(argTimeoutProgress, func(done chan<- struct{}, errs chan<- error) {
		for {
			res, err := Request("GET", "progress", nil)

			if err != nil {
				errs <- err
				return
			}

			defer res.Body.Close()

			if res.StatusCode == http.StatusAccepted {
				logrus.Info("StepProgress: Build is pending. Retry")
				time.Sleep(5 * time.Second)
				continue
			}

			if res.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("StepProgress: Got an unexpected response %d while expecting %d. Exiting", res.StatusCode, http.StatusOK)
				return
			}

			_, err = io.Copy(os.Stdout, res.Body)
			if err != nil {
				errs <- fmt.Errorf("StepProgress: Unable to write response body to stdout: %v", err)
			}

			break
		}

		logrus.Info("StepProgress: Done")

		close(done)
	})
}

func StepExport() error {
	return Wait(argTimeoutExport, func(done chan<- struct{}, errs chan<- error) {
		for _, export := range argExports {
			res, err := Request("GET", fmt.Sprintf("export?path=%s", url.PathEscape(export)), nil)

			if err != nil {
				errs <- err
				return
			}

			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("StepExport: Got an unexpected response %d while expecting %d. Exiting", res.StatusCode, http.StatusOK)
				return
			}

			dstPath := path.Join(argOutputPath, export)
			dstDir := filepath.Dir(dstPath)

			if _, err := os.Stat(dstDir); os.IsNotExist(err) {
				logrus.Infof("StepExport: Destination directory does not exist. Creating %s", dstDir)
				if err := os.MkdirAll(dstDir, 0700); err != nil {
					errs <- fmt.Errorf("StepExport: Failed to create destination directory: %s", err)
				}
			}

			dst, err := os.OpenFile(
				path.Join(argOutputPath, export),
				os.O_WRONLY|os.O_CREATE|os.O_EXCL,
				0400,
			)

			if err != nil {
				errs <- fmt.Errorf("StepExport: Failed to open destination response: %s", err)
				return
			}

			_, err = io.Copy(dst, res.Body)

			if err != nil {
				errs <- fmt.Errorf("StepExport: Failed to copy response: %s", err)
				return
			}
		}

		logrus.Info("StepExport: Done")

		close(done)
	})
}
