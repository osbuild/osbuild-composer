package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func handleResult(logger *logrus.Logger, config *Config) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			logger.Debugf("handlerResult called on %s", r.URL.Path)
			if r.Method != http.MethodGet {
				http.Error(w, "result endpoint only supports Get", http.StatusMethodNotAllowed)
				return
			}
			buildResult := newBuildResult(config)
			switch {
			case buildResult.Bad():
				http.Error(w, "build failed", http.StatusBadRequest)
				f, err := os.Open(filepath.Join(config.BuildDirBase, "build/build.log"))
				if err != nil {
					logger.Errorf("cannot open log: %v", err)
					return
				}
				defer f.Close()
				io.Copy(w, f)
				return
			case buildResult.Good():
				// good result
			default:
				http.Error(w, "build still running", http.StatusTooEarly)
				return
			}

			fss := http.FileServer(http.Dir(filepath.Join(config.BuildDirBase, "build/output")))
			fss.ServeHTTP(w, r)
		},
	)
}
