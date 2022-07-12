package osbuild

import (
	"encoding/json"
	"errors"

	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

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
		case "org.osbuild.curl":
			source = new(CurlSource)
		case "org.osbuild.inline":
			source = new(InlineSource)
		case "org.osbuild.ostree":
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

func GenSources(packages []rpmmd.PackageSpec, ostreeCommits []ostree.CommitSource, inlineData []string) Sources {
	sources := Sources{}
	curl := &CurlSource{
		Items: make(map[string]CurlSourceItem),
	}
	for _, pkg := range packages {
		item := new(CurlSourceOptions)
		item.URL = pkg.RemoteLocation
		if pkg.Secrets == "org.osbuild.rhsm" {
			item.Secrets = &URLSecrets{
				Name: "org.osbuild.rhsm",
			}
		}
		item.Insecure = pkg.IgnoreSSL
		curl.Items[pkg.Checksum] = item
	}
	if len(curl.Items) > 0 {
		sources["org.osbuild.curl"] = curl
	}

	ostree := &OSTreeSource{
		Items: make(map[string]OSTreeSourceItem),
	}
	for _, commit := range ostreeCommits {
		item := new(OSTreeSourceItem)
		item.Remote.URL = commit.URL
		ostree.Items[commit.Checksum] = *item
	}
	if len(ostree.Items) > 0 {
		sources["org.osbuild.ostree"] = ostree
	}

	if len(inlineData) > 0 {
		ils := NewInlineSource()
		for _, data := range inlineData {
			ils.AddItem(data)
		}

		sources["org.osbuild.inline"] = ils
	}

	return sources
}
