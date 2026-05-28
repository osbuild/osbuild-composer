package flatpak

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

type ResponseImageList struct{}

// OCIRegistryIndex is a session for querying one OCI registry's Flatpak static index
// (/index/static) for a fixed os/tag pair. The decoded JSON is cached on this instance
// until [OCIRegistryIndex.Close]. Callers should defer Close() once they are done
// issuing [OCIRegistryIndex.Query] calls.
//
// Manifest and config resolution for the digest-pinned image is delegated to
// [container.Resolver] from pkg/container ([container.NewBlockingResolver]).
type OCIRegistryIndex struct {
	baseURI string
	os      string
	tag     string

	mu       sync.Mutex
	cacheKey string
	root     *ResponseRoot
}

// NewOCIRegistryIndex constructs an index client for baseURI (https host or full origin
// without the oci+ scheme prefix), Flatpak index os and tag query parameters.
func NewOCIRegistryIndex(baseURI, os, tag string) (*OCIRegistryIndex, error) {
	if baseURI == "" {
		return nil, fmt.Errorf("flatpak oci registry: empty base URI")
	}
	return &OCIRegistryIndex{baseURI: baseURI, os: os, tag: tag}, nil
}

// Close drops the cached decoded index. It must not be called concurrently with Query.
func (q *OCIRegistryIndex) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cacheKey = ""
	q.root = nil
}

// Query parses flatpakRef with [NewReferenceFromString] for architecture selection,
// loads the registry index (cached on this instance), finds the image by
// org.flatpak.ref, then resolves the digest-pinned image using [container.NewBlockingResolver]
// (pkg/container).
func (q *OCIRegistryIndex) Query(flatpakRef string) (*container.Spec, error) {
	parsed, err := NewReferenceFromString(flatpakRef)
	if err != nil {
		return nil, err
	}

	res, err := q.getResponseRoot()
	if err != nil {
		return nil, err
	}

	repoName, manifestDigest, err := findFlatpakInIndex(res, flatpakRef)
	if err != nil {
		return nil, err
	}

	imageRef := ociImageRefFromIndexComponents(res.Registry, repoName, manifestDigest)

	r := container.NewBlockingResolver(parsed.Arch)
	spec, err := r.Resolve(container.SourceSpec{
		Source:    imageRef,
		Name:      repoName,
		TLSVerify: nil,
		Local:     false,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve flatpak container %q: %w", imageRef, err)
	}
	return &spec, nil
}

func (q *OCIRegistryIndex) buildIndexGETRequest() (*http.Request, string, error) {
	indexURL, err := url.JoinPath(q.baseURI, ENDPOINT_STATIC)
	if err != nil {
		return nil, "", fmt.Errorf("could not format URI: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, "", err
	}

	// The default set of query parameters that are always passed by
	// flatpak. See [1].
	//
	// [1]: https://github.com/flatpak/flatpak-oci-specs/blob/main/registry-index.md#appendix---usage-by-flatpak
	query := req.URL.Query()
	query.Set("label:org.flatpak.ref:exists", "1")
	query.Set("os", q.os)
	query.Set("tag", q.tag)
	req.URL.RawQuery = query.Encode()

	return req, req.URL.String(), nil
}

func (q *OCIRegistryIndex) getResponseRoot() (*ResponseRoot, error) {
	req, cacheKey, err := q.buildIndexGETRequest()
	if err != nil {
		return nil, err
	}

	q.mu.Lock()
	if q.cacheKey == cacheKey && q.root != nil {
		root := q.root
		q.mu.Unlock()
		return root, nil
	}
	q.mu.Unlock()

	client, err := httpClient()
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry index %q returned status: %s", cacheKey, res.Status)
	}

	root := new(ResponseRoot)
	if err := json.NewDecoder(res.Body).Decode(root); err != nil {
		return nil, err
	}

	q.mu.Lock()
	q.cacheKey = cacheKey
	q.root = root
	q.mu.Unlock()

	return root, nil
}

// findFlatpakInIndex returns the repository name and manifest digest for the first image
// whose org.flatpak.ref label equals wantRef.
func findFlatpakInIndex(res *ResponseRoot, wantRef string) (repoName, manifestDigest string, err error) {
	for _, result := range res.Results {
		for _, img := range result.Images {
			if img == nil || img.Labels == nil {
				continue
			}
			if img.Labels["org.flatpak.ref"] != wantRef {
				continue
			}
			if res.Registry == "" {
				return "", "", fmt.Errorf("registry index: found %q but Registry field was missing", wantRef)
			}
			return result.Name, img.Digest, nil
		}
	}
	return "", "", fmt.Errorf("did not find image %q", wantRef)
}

// ociImageRefFromIndexComponents builds a docker-style reference host/repo@digest for
// [container.NewClient] / the blocking resolver.
func ociImageRefFromIndexComponents(registryURL, repoName, manifestDigest string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(registryURL, "https://"), "http://")
	host = strings.TrimSuffix(host, "/")
	repoPath := strings.TrimPrefix(repoName, "/")
	return fmt.Sprintf("%s/%s@%s", host, repoPath, manifestDigest)
}

func httpClient() (*http.Client, error) {
	return &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
		Timeout:   300 * time.Second,
	}, nil
}
