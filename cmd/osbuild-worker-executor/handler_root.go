package main

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

func handleRoot(logger *logrus.Logger, _ *Config) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// we just return ok here
			logger.Info("/ handler called")
		},
	)
}
