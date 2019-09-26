package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"

	"osbuild-composer/internal/queue"
	"osbuild-composer/internal/rpmmd"
	"osbuild-composer/internal/weldr"
)

const StateFile = "/var/lib/osbuild-composer/weldr-state.json"

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "v", false, "Print access log")
	flag.Parse()

	err := os.Remove("/run/weldr/api.socket")
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	err = os.Mkdir("/run/weldr", 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	listener, err := net.Listen("unix", "/run/weldr/api.socket")
	if err != nil {
		panic(err)
	}

	repo := rpmmd.RepoConfig{
		Id:       "fedora-30",
		Name:     "Fedora 30",
		Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-30&arch=x86_64",
	}

	packages, err := rpmmd.FetchPackageList(repo)
	if err != nil {
		panic(err)
	}

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	err = os.MkdirAll("/var/lib/osbuild-composer", 0755)
	if err != nil {
		panic(err)
	}

	state, err := ioutil.ReadFile(StateFile)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("cannot read state: %v", err)
	}

	stateChannel := make(chan []byte, 10)
	buildChannel := make(chan queue.Build, 200)
	api := weldr.New(repo, packages, logger, state, stateChannel, buildChannel)
	go func() {
		for {
			err := writeFileAtomically(StateFile, <-stateChannel, 0755)
			if err != nil {
				log.Fatalf("cannot write state: %v", err)
			}
		}
	}()

	api.Serve(listener)
}

func writeFileAtomically(filename string, data []byte, mode os.FileMode) error {
	dir, name := filepath.Dir(filename), filepath.Base(filename)

	tmpfile, err := ioutil.TempFile(dir, name+"-*.tmp")
	if err != nil {
		return err
	}

	_, err = tmpfile.Write(data)
	if err != nil {
		os.Remove(tmpfile.Name())
		return err
	}

	err = tmpfile.Chmod(mode)
	if err != nil {
		return err
	}

	err = tmpfile.Close()
	if err != nil {
		os.Remove(tmpfile.Name())
		return err
	}

	err = os.Rename(tmpfile.Name(), filename)
	if err != nil {
		os.Remove(tmpfile.Name())
		return err
	}

	return nil
}
