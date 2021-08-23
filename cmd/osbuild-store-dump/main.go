// This fills and saves a store with more or less arbitrary data. It is meant to generate test stores as
// test data for testing upgrades to composer.
package main

import (
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
)

func getManifest(bp blueprint.Blueprint, t distro.ImageType, a distro.Arch, d distro.Distro, rpm_md rpmmd.RPMMD, repos []rpmmd.RepoConfig) distro.Manifest {
	packageSets := t.PackageSets(bp)
	pkgSpecSets := make(map[string][]rpmmd.PackageSpec)
	for name, packages := range packageSets {
		pkgs, _, err := rpm_md.Depsolve(packages, repos, d.ModulePlatformID(), a.Name(), d.Releasever())
		if err != nil {
			panic(err)
		}
		pkgSpecSets[name] = pkgs
	}
	manifest, err := t.Manifest(bp.Customizations, distro.ImageOptions{}, repos, pkgSpecSets, 0)
	if err != nil {
		panic(err)
	}

	return manifest
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	id1, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	id2, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	hostname := "my-host"
	description := "Mostly harmless."
	password := "password"
	sshKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC61wMCjOSHwbVb4VfVyl5sn497qW4PsdQ7Ty7aD6wDNZ/QjjULkDV/yW5WjDlDQ7UqFH0Sr7vywjqDizUAqK7zM5FsUKsUXWHWwg/ehKg8j9xKcMv11AkFoUoujtfAujnKODkk58XSA9whPr7qcw3vPrmog680pnMSzf9LC7J6kXfs6lkoKfBh9VnlxusCrw2yg0qI1fHAZBLPx7mW6+me71QZsS6sVz8v8KXyrXsKTdnF50FjzHcK9HXDBtSJS5wA3fkcRYymJe0o6WMWNdgSRVpoSiWaHHmFgdMUJaYoCfhXzyl7LtNb3Q+Sveg+tJK7JaRXBLMUllOlJ6ll5Hod root@localhost"
	home := "/home/my-home"
	shell := "/bin/true"
	uid := 42
	gid := 42
	bp1 := blueprint.Blueprint{
		Name:        "my-blueprint-1",
		Description: "My first blueprint",
		Packages: []blueprint.Package{
			{
				Name: "tmux",
			},
		},
		Groups: []blueprint.Group{
			{
				Name: "core",
			},
		},
	}
	bp2 := blueprint.Blueprint{
		Name:        "my-blueprint-2",
		Description: "My second blueprint",
		Version:     "0.0.2",
		Customizations: &blueprint.Customizations{
			Hostname: &hostname,
			Kernel: &blueprint.KernelCustomization{
				Append: "debug",
			},
			SSHKey: []blueprint.SSHKeyCustomization{
				{
					User: "me",
					Key:  sshKey,
				},
			},
			User: []blueprint.UserCustomization{
				{
					Name:        "myself",
					Description: &description,
					Password:    &password,
					Key:         &sshKey,
					Home:        &home,
					Shell:       &shell,
					Groups: []string{
						"wheel",
					},
					UID: &uid,
					GID: &gid,
				},
			},
		},
	}
	awsTarget := target.NewAWSTarget(
		&target.AWSTargetOptions{
			Filename:        "image.ami",
			Region:          "far-away-1",
			AccessKeyID:     "MyKey",
			SecretAccessKey: "MySecret",
			Bucket:          "list",
			Key:             "image",
		},
	)
	awsTarget.Uuid = id1
	awsTarget.ImageName = "My Image"
	awsTarget.Created = time.Now()

	d := fedora32.New()
	a, err := d.GetArch("x86_64")
	if err != nil {
		panic(err)
	}
	t1, err := a.GetImageType("qcow2")
	if err != nil {
		panic(err)
	}
	t2, err := a.GetImageType("fedora-iot-commit")
	if err != nil {
		panic(err)
	}
	allRepos, err := rpmmd.LoadRepositories([]string{cwd}, "fedora-32")
	if err != nil {
		panic(err)
	}
	repos := allRepos["x86_64"]
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic("os.UserHomeDir(): " + err.Error())
	}
	rpmmd := rpmmd.NewRPMMD(path.Join(homeDir, ".cache/osbuild-composer/rpmmd"), "/usr/libexec/osbuild-composer/dnf-json")

	s := store.New(&cwd, a, nil)
	if s == nil {
		panic("could not create store")
	}
	err = s.PushBlueprint(bp1, "message 1")
	if err != nil {
		panic(err)
	}
	err = s.PushBlueprint(bp1, "message 2")
	if err != nil {
		panic(err)
	}
	err = s.PushBlueprintToWorkspace(bp2)
	if err != nil {
		panic(err)
	}
	err = s.PushCompose(id1,
		getManifest(bp2, t1, a, d, rpmmd, repos),
		t1,
		&bp2,
		0,
		[]*target.Target{
			awsTarget,
		},
		id1,
	)
	if err != nil {
		panic(err)
	}
	err = s.PushCompose(id2,
		getManifest(bp2, t2, a, d, rpmmd, repos),
		t2,
		&bp2,
		0,
		[]*target.Target{
			awsTarget,
		},
		id2,
	)
	if err != nil {
		panic(err)
	}
}
