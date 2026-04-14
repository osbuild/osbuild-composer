package flatpak

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/osbuild/images/pkg/container"
)

// Functionality query for flatpaks in an OCI registry that supports the registry index
// specification [1], for example quay.io or any other repository based on pulp.
//
// We try to be as similar to Flatpak in our queries as we can be.
//
// [1]: https://github.com/flatpak/flatpak-oci-specs/blob/main/registry-index.md

const (
	// Flatpak only uses the static endpoint
	ENDPOINT_STATIC = "/index/static"
)

type ResponseRoot struct {
	Registry string               `json:"Registry"`
	Results  []ResponseRepository `json:"Results"`
}

type ResponseRepository struct {
	Name   string              `json:"Name"`
	Images []*ResponseImage    `json:"Images"`
	Lists  []ResponseImageList `json:"Lists"`
}

type ResponseImage struct {
	Tags         []string          `json:"Tags"`
	Digest       string            `json:"Digest"`
	MediaType    string            `json:"MediaType"`
	OS           string            `json:"OS"`
	Architecture string            `json:"Architecture"`
	Annotations  map[string]string `json:"Annotations"`
	Labels       map[string]string `json:"Labels"`
}

// Deserialization of an OCI Manifest, only defines the fields necessary for us
// to get what we need; which is the digest of the config object.
type OCIManifest struct {
	Config *struct {
		Digest string `json:"digest,omitempty"`
	} `json:"config,omitempty"`
}

type ResponseImageList struct{}

func QueryOCIRegistryIndex(uri, ref, os, tag string) (*container.Spec, error) {
	res, err := fetchRegistryIndex(uri, os, tag)
	if err != nil {
		return nil, err
	}

	for _, result := range res.Results {
		for _, img := range result.Images {
			if img.Labels["org.flatpak.ref"] == ref {
				imageID, err := fetchDockerAPIConfigDigest(res.Registry, result.Name, img.Digest)
				if err != nil {
					return nil, err
				}

				reg, _ := strings.CutPrefix(res.Registry, "https://")

				return &container.Spec{
					Source:    fmt.Sprintf("%s%s", reg, result.Name),
					Digest:    img.Digest,
					ImageID:   imageID,
					LocalName: result.Name,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("did not find image %q", ref)
}

func httpClient() (*http.Client, error) {
	return &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
		Timeout:   300 * time.Second,
	}, nil
}

func fetchRegistryIndex(uri, os, tag string) (*ResponseRoot, error) {
	client, err := httpClient()
	if err != nil {
		return nil, err
	}

	uri, err = url.JoinPath(uri, ENDPOINT_STATIC)
	if err != nil {
		return nil, fmt.Errorf("could not format URI: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	// The default set of query parameters that are always passed by
	// flatpak. See [1].
	//
	// [1]: https://github.com/flatpak/flatpak-oci-specs/blob/main/registry-index.md#appendix---usage-by-flatpak
	q := req.URL.Query()
	q.Set("label:org.flatpak.ref:exists", "1")
	q.Set("os", os)
	q.Set("tag", tag)
	req.URL.RawQuery = q.Encode()

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry index %q returned status: %s", uri, res.Status)
	}

	var root ResponseRoot
	if err := json.NewDecoder(res.Body).Decode(&root); err != nil {
		return nil, err
	}

	return &root, nil
}

// Use the docker API provided by a registry to fetch the digest of the 'config' section of
// a manifest [1], [2].
// [1]: https://distribution.github.io/distribution/spec/api/#pulling-an-image-manifest
// [2]: https://specs.opencontainers.org/image-spec/manifest/
func fetchDockerAPIConfigDigest(registry, name, digest string) (string, error) {
	client, err := httpClient()
	if err != nil {
		return "", err
	}

	uri, err := url.JoinPath(registry, "v2", name, "manifests", digest)
	if err != nil {
		return "", fmt.Errorf("could not format URI: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("docker api error %d %w %q", res.StatusCode, err, uri)
	}

	var manifest OCIManifest
	if err := json.NewDecoder(res.Body).Decode(&manifest); err != nil {
		return "", err
	}

	if manifest.Config == nil {
		return "", fmt.Errorf("manifest did not contain config")
	}

	if manifest.Config.Digest == "" {
		return "", fmt.Errorf("config did not contain digest")
	}

	return manifest.Config.Digest, nil
}
