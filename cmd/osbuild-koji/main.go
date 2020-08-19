package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
)

func main() {
	var server, user, password, name, version, release, arch, filename string
	flag.StringVar(&server, "server", "", "url to API")
	flag.StringVar(&user, "user", "", "koji username")
	flag.StringVar(&password, "password", "", "koji password")
	flag.StringVar(&name, "name", "", "image name")
	flag.StringVar(&version, "version", "", "image verison")
	flag.StringVar(&release, "release", "", "image release")
	flag.StringVar(&arch, "arch", "", "image architecture")
	flag.StringVar(&filename, "filename", "", "filename")
	flag.Parse()

	id, err := uuid.NewRandom()
	if err != nil {
		println(err.Error())
		return
	}
	dir := fmt.Sprintf("osbuild-%v", id)

	file, err := os.Open(filename)
	if err != nil {
		println(err.Error())
		return
	}
	defer file.Close()

	k, err := koji.Login(server, "osbuild", "osbuildpass", http.DefaultTransport)
	if err != nil {
		println(err.Error())
		return
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			log.Print("logging out of koji failed ", err)
		}
	}()

	hash, length, err := k.Upload(file, dir, path.Base(filename))
	if err != nil {
		println(err.Error())
		return
	}

	build := koji.Build{
		Name:      name,
		Version:   version,
		Release:   release,
		StartTime: time.Now().Unix(),
		EndTime:   time.Now().Unix(),
	}
	buildRoots := []koji.BuildRoot{
		{
			ID: 1,
			Host: koji.Host{
				Os:   "RHEL8",
				Arch: arch,
			},
			ContentGenerator: koji.ContentGenerator{
				Name:    "osbuild",
				Version: "1",
			},
			Container: koji.Container{
				Type: "nspawn",
				Arch: arch,
			},
			Tools:      []koji.Tool{},
			Components: []koji.Component{},
		},
	}
	output := []koji.Output{
		{
			BuildRootID:  1,
			Filename:     path.Base(filename),
			FileSize:     length,
			Arch:         arch,
			ChecksumType: "md5",
			MD5:          hash,
			Type:         "image",
			Components:   []koji.Component{},
			Extra: koji.OutputExtra{
				Image: koji.OutputExtraImageInfo{
					Arch: arch,
				},
			},
		},
	}

	result, err := k.CGImport(build, buildRoots, output, dir)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Printf("Success, build id: %d\n", result.BuildID)
}
