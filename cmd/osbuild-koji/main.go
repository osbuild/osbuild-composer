package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
)

func main() {
	var taskID int
	var server, user, password, name, version, release, arch, filename string
	flag.IntVar(&taskID, "task-id", 0, "id of owning task")
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

	transport := koji.CreateRetryableTransport()
	k, err := koji.NewFromPlain(server, "osbuild", "osbuildpass", transport)
	if err != nil {
		println(err.Error())
		return
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			log.Printf("logging out of koji failed: %s ", err)
		}
	}()

	hash, length, err := k.Upload(file, dir, path.Base(filename))
	if err != nil {
		println(err.Error())
		return
	}

	build := koji.Build{
		TaskID:    uint64(taskID),
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
			Tools: []koji.Tool{},
			RPMs:  []rpmmd.RPM{},
		},
	}
	output := []koji.BuildOutput{
		{
			BuildRootID:  1,
			Filename:     path.Base(filename),
			FileSize:     length,
			Arch:         arch,
			ChecksumType: koji.ChecksumTypeMD5,
			Checksum:     hash,
			Type:         koji.BuildOutputTypeImage,
			RPMs:         []rpmmd.RPM{},
			Extra: &koji.BuildOutputExtra{
				ImageOutput: koji.ImageExtraInfo{
					Arch:     arch,
					BootMode: platform.BOOT_NONE.String(), // TODO: put the correct boot mode here
				},
			},
		},
	}

	initResult, err := k.CGInitBuild(build.Name, build.Version, build.Release)
	if err != nil {
		println(err.Error())
		return
	}

	build.BuildID = uint64(initResult.BuildID)

	importResult, err := k.CGImport(build, buildRoots, output, dir, initResult.Token)
	if err != nil {
		println(err.Error())
		return
	}

	fmt.Printf("Success, build id: %d\n", importResult.BuildID)
}
