package main

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

func addRoutes(mux *http.ServeMux, logger *logrus.Logger, config *Config) {
	mux.Handle("/api/v1/build", handleBuild(logger, config))
	mux.Handle("/api/v1/result/", http.StripPrefix("/api/v1/result/", handleResult(logger, config)))
	mux.Handle("/api/v1/log", http.StripPrefix("/api/v1/log", handleLog(logger, config)))
	mux.Handle("/", handleRoot(logger, config))
}
