package main

import (
	"io"

	"github.com/BurntSushi/toml"
)

type ComposerConfigFile struct {
	Koji struct {
		AllowedDomains []string `toml:"allowed_domains"`
		CA             string   `toml:"ca"`
	} `toml:"koji"`
	Worker struct {
		AllowedDomains []string `toml:"allowed_domains"`
		CA             string   `toml:"ca"`
	} `toml:"worker"`
	ComposerAPI struct {
		IdentityFilter []string `toml:"identity_filter"`
	} `toml:"composer_api"`
	WorkerAPI struct {
		IdentityFilter []string `toml:"identity_filter"`
	} `toml:"worker_api"`
}

func LoadConfig(name string) (*ComposerConfigFile, error) {
	var c ComposerConfigFile
	_, err := toml.DecodeFile(name, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func DumpConfig(c *ComposerConfigFile, w io.Writer) error {
	return toml.NewEncoder(w).Encode(c)
}
