package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func handleLog(logger *logrus.Logger, config *Config) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			logger.Debugf("handlerLog called on %s", r.URL.Path)
			if r.Method != http.MethodGet {
				http.Error(w, "result endpoint only supports Get", http.StatusMethodNotAllowed)
				return
			}

			var f *os.File
			var err error
			buildResult := newBuildResult(config)
			switch {
			case buildResult.Bad():
				// result will not have been moved to output directory
				f, err = os.Open(filepath.Join(config.BuildDirBase, "build/osbuild-result.json"))
				if err != nil {
					logger.Errorf("cannot open log: %v", err)
					http.Error(w, fmt.Sprintf("unable to read log: %v", err), http.StatusInternalServerError)
					return
				}
				defer f.Close()
			case buildResult.Good():
				f, err = os.Open(filepath.Join(config.BuildDirBase, "build/output/osbuild-result.json"))
				if err != nil {
					logger.Errorf("cannot open log: %v", err)
					http.Error(w, fmt.Sprintf("unable to read log: %v", err), http.StatusInternalServerError)
					return
				}
				defer f.Close()
			default:
				http.Error(w, "build still running", http.StatusTooEarly)
				return
			}
			if _, err := io.Copy(w, f); err != nil {
				logger.Errorf("Unable to write log to response")
			}
		},
	)
}
