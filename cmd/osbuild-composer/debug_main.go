// +build debug

package main

import (
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/rcm"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
)

func setUpUDSListener(path string) net.Listener {
	if err := os.RemoveAll(path); err != nil {
		log.Panic(err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		log.Panic("listen error:", err)
	}
	return l
}

func setUpTCPListener(socket string) net.Listener {
	l, err := net.Listen("tcp", socket)
	if err != nil {
		log.Panic("listen error:", err)
	}
	return l
}

func main() {
	// Set up temp directory
	dir, err := ioutil.TempDir("", "osbuild-composer-debug-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Set environment variable which are set by systemd in production
	stateDir := filepath.Join(dir, "state")
	_ = os.Mkdir(stateDir, 0700)

	cacheDir := filepath.Join(dir, "cache")
	_ = os.Mkdir(cacheDir, 0700)

	_ = os.Setenv("STATE_DIRECTORY", stateDir)
	_ = os.Setenv("CACHE_DIRECTORY", stateDir)

	// Create all necessary distro configurations
	rpm, distribution, distros := createDistroConfiguration([]string{"."})
	store := store.New(&stateDir, distribution, *distros)

	// Run all the APIs
	logger := log.New(os.Stdout, "", 0)

	// UDS only Job API
	jobListener := setUpUDSListener(filepath.Join(dir, "job.socket"))
	defer jobListener.Close()
	jobAPI := jobqueue.New(logger, store)
	go func() {
		err := jobAPI.Serve(jobListener)
		common.PanicOnError(err)
	}()

	// RCM API running on localhost
	rcmListener := setUpTCPListener("127.0.0.1:8080")
	defer rcmListener.Close()
	rcmAPI := rcm.New(logger, store, rpm)
	go func() {
		err := rcmAPI.Serve(rcmListener)
		// If the RCM API fails, take down the whole process, not just a single gorutine
		log.Fatal("RCM API failed: ", err)
	}()

	weldrListener := setUpUDSListener(filepath.Join(dir, "weldr.socket"))
	defer weldrListener.Close()
	weldrAPI := weldr.New(rpm, common.CurrentArch(), distribution, logger, store)
	err = weldrAPI.Serve(weldrListener)
	common.PanicOnError(err)
}