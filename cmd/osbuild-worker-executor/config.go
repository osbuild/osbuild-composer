package main

import (
	"flag"
)

type Config struct {
	Host string
	Port string

	BuildDirBase string
}

func newConfigFromCmdline(args []string) (*Config, error) {
	var config Config

	fs := flag.NewFlagSet("oaas", flag.ContinueOnError)
	fs.StringVar(&config.Host, "host", "localhost", "host to listen on")
	fs.StringVar(&config.Port, "port", "8001", "port to listen on")
	fs.StringVar(&config.BuildDirBase, "build-path", "/var/tmp/oaas", "base dir to run the builds in")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return &config, nil
}
