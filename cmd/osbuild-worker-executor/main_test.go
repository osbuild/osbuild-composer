package main_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker-executor"
)

const defaultTimeout = 5 * time.Second

func waitReady(ctx context.Context, timeout time.Duration, endpoint string) error {
	client := http.Client{}
	startTime := time.Now()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		resp.Body.Close()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if time.Since(startTime) >= timeout {
				return fmt.Errorf("timeout reached while waiting for endpoint")
			}
			// wait a little while between checks
			time.Sleep(250 * time.Millisecond)
		}
	}
}

func runTestServer(t *testing.T) (baseURL, buildBaseDir string, loggerHook *logrusTest.Hook) {
	host := "localhost"
	port := "18002"
	buildBaseDir = t.TempDir()
	baseURL = fmt.Sprintf("http://%s:%s/", host, port)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger, loggerHook := logrusTest.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	args := []string{
		"-host", host,
		"-port", port,
		"-build-path", buildBaseDir,
	}
	go func() {
		_ = main.Run(ctx, args, os.Getenv, logger)
	}()

	err := waitReady(ctx, defaultTimeout, baseURL)
	assert.NoError(t, err)

	return baseURL, buildBaseDir, loggerHook
}
