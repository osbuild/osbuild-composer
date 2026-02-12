package osbuild

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/hashutil"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
)

// RpmDownloader specifies what backend to use for rpm downloads
// Note that the librepo backend requires osbuild v138 from
// 2025-01-15.
type RpmDownloader uint64

const (
	RpmDownloaderCurl = iota
	RpmDownloaderLibrepo
)

// SourceInputs contains the inputs to generate osbuild.Sources
// Note that for Packages/RpmRepos the depsolve resolved results
// must be passed
type SourceInputs struct {
	Depsolved  depsolvednf.DepsolveResult
	Containers []container.Spec
	Commits    []ostree.CommitSpec
	// InlineData contans the inline data for fsnode.Files
	InlineData []string
	// FileRefs contains the references of paths/urls for fsnode.Files
	FileRefs []string
}

// A Sources map contains all the sources made available to an osbuild run
type Sources map[string]Source

// Source specifies the operations of a given source-type.
type Source interface {
	isSource()
}

type SourceOptions interface {
	isSourceOptions()
}

type rawSources map[string]json.RawMessage

// UnmarshalJSON unmarshals JSON into a Source object. Each type of source has
// a custom unmarshaller for its options, selected based on the source name.
func (sources *Sources) UnmarshalJSON(data []byte) error {
	var rawSources rawSources
	err := json.Unmarshal(data, &rawSources)
	if err != nil {
		return err
	}
	*sources = make(map[string]Source)
	for name, rawSource := range rawSources {
		var source Source
		switch name {
		case SourceNameCurl:
			source = new(CurlSource)
		case SourceNameLibrepo:
			source = new(LibrepoSource)
		case SourceNameInline:
			source = new(InlineSource)
		case SourceNameOstree:
			source = new(OSTreeSource)
		default:
			return errors.New("unexpected source name: " + name)
		}
		err = json.Unmarshal(rawSource, source)
		if err != nil {
			return err
		}
		(*sources)[name] = source
	}

	return nil
}

func (sources Sources) addPackagesCurl(packages rpmmd.PackageList) error {
	curl := NewCurlSource()
	for _, pkg := range packages {
		err := curl.AddPackage(pkg)
		if err != nil {
			return err
		}
	}
	sources[SourceNameCurl] = curl
	return nil
}

func (sources Sources) addPackagesLibrepo(packages rpmmd.PackageList, rpmRepos []rpmmd.RepoConfig) error {
	librepo := NewLibrepoSource()
	for _, pkg := range packages {
		err := librepo.AddPackage(pkg, rpmRepos)
		if err != nil {
			return err
		}
	}
	sources[SourceNameLibrepo] = librepo
	return nil
}

// GenSources generates the Sources from the given inputs. Note that
// the packages and rpmRepos need to come from the *resolved* set.
func GenSources(inputs SourceInputs, rpmDownloader RpmDownloader) (Sources, error) {
	sources := Sources{}

	// collect rpm package sources
	if len(inputs.Depsolved.Transactions.AllPackages()) > 0 {
		var err error
		switch rpmDownloader {
		case RpmDownloaderCurl:
			err = sources.addPackagesCurl(inputs.Depsolved.Transactions.AllPackages())
		case RpmDownloaderLibrepo:
			err = sources.addPackagesLibrepo(inputs.Depsolved.Transactions.AllPackages(), inputs.Depsolved.Repos)
		default:
			err = fmt.Errorf("unknown rpm downloader %v", rpmDownloader)
		}
		if err != nil {
			return nil, err
		}
	}

	// collect ostree commit sources
	if len(inputs.Commits) > 0 {
		ostree := NewOSTreeSource()
		for _, commit := range inputs.Commits {
			ostree.AddItem(commit)
		}
		if len(ostree.Items) > 0 {
			sources[SourceNameOstree] = ostree
		}
	}

	// collect inline data sources
	if len(inputs.InlineData) > 0 {
		ils := NewInlineSource()
		for _, data := range inputs.InlineData {
			ils.AddItem(data)
		}

		sources[SourceNameInline] = ils
	}

	// collect skopeo and local container sources
	if len(inputs.Containers) > 0 {
		skopeo := NewSkopeoSource()
		skopeoIndex := NewSkopeoIndexSource()
		localContainers := NewContainersStorageSource()
		for _, c := range inputs.Containers {
			if c.LocalStorage {
				localContainers.AddItem(c.ImageID)
			} else {
				skopeo.AddItem(c.Source, c.Digest, c.ImageID, c.TLSVerify)
				// if we have a list digest, add a skopeo-index source as well
				if c.ListDigest != "" {
					skopeoIndex.AddItem(c.Source, c.ListDigest, c.TLSVerify)
				}
			}
		}
		if len(skopeo.Items) > 0 {
			sources[SourceNameSkopeo] = skopeo
		}
		if len(skopeoIndex.Items) > 0 {
			sources[SourceNameSkopeoIndex] = skopeoIndex
		}
		if len(localContainers.Items) > 0 {
			sources[SourceNameContainersStorage] = localContainers
		}
	}

	// collect host resources
	if len(inputs.FileRefs) > 0 {
		// XXX: fugly
		curl, ok := sources["org.osbuild.curl"].(*CurlSource)
		if !ok || curl == nil {
			curl = NewCurlSource()
		}
		for _, hostRes := range inputs.FileRefs {
			checksum, err := hashutil.Sha256sum(hostRes)
			if err != nil {
				return nil, err
			}
			curl.Items["sha256:"+checksum] = &CurlSourceOptions{
				URL: fmt.Sprintf("file:%s", hostRes),
			}
		}
		sources["org.osbuild.curl"] = curl
	}

	return sources, nil
}
