package logger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSplunkLogger(t *testing.T) {
	ch := make(chan bool)
	time.AfterFunc(time.Second*10, func() {
		ch <- false
	})
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// make sure the logger retries requests
		if count == 0 {
			count += 1
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		require.Equal(t, "Splunk", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var sp SplunkPayload
		err := json.NewDecoder(r.Body).Decode(&sp)
		require.NoError(t, err)
		require.Equal(t, "test-host", sp.Host)
		require.Equal(t, "test-host", sp.Event.Host)
		require.Equal(t, "image-builder", sp.Event.Ident)
		require.Equal(t, "message", sp.Event.Message)
		ch <- true
	}))
	sl := NewSplunkLogger(context.Background(), srv.URL, "", "image-builder", "test-host")
	require.NoError(t, sl.LogWithTime(time.Now(), "message"))
	require.True(t, <-ch)
}

func TestSplunkLoggerContext(t *testing.T) {
	ch := make(chan bool)
	time.AfterFunc(time.Second*10, func() {
		ch <- false
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Splunk", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var sp SplunkPayload
		err := json.NewDecoder(r.Body).Decode(&sp)
		require.NoError(t, err)
		require.Equal(t, "test-host", sp.Host)
		require.Equal(t, "test-host", sp.Event.Host)
		require.Equal(t, "image-builder", sp.Event.Ident)
		require.Equal(t, "message", sp.Event.Message)
		ch <- true
	}))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	sl := NewSplunkLogger(ctx, srv.URL, "", "image-builder", "test-host")
	require.NoError(t, sl.LogWithTime(time.Now(), "message"))
	require.True(t, <-ch)
}
