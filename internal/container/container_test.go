package container_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
)

const rootLayer = `H4sIAAAJbogA/+SWUYqDMBCG53lP4V5g9x8dzRX2Bvtc0VIhEIhKe/wSKxgU6ktjC/O9hMzAQDL8
/8yltdb9DLeB0gEGKhHCg/UJsBAL54zKFBAC54ZzyrCUSMfYDydPgHfu6R/s5VePilOfzF/of/bv
vG2+lqhyFNGPddP53yjyegCBKcuNROZ77AmBoP+CmbIyqpEM5fqf+3/ubJtsCuz7P1b+L1Du/4f5
v+vrsVPu/Vq9P3ANk//d+x/MZv8TKNf/Qfqf9v9v5fLXK3/lKEc5ypm4AwAA//8DAE6E6nIAEgAA
`

// The following code implements a toy container registry to test with

// Blob interface
type Blob interface {
	GetSize() int64
	GetMediaType() string
	GetDigest() digest.Digest

	Reader() io.Reader
}

// dataBlob //
type dataBlob struct {
	Data      []byte
	MediaType string
}

func NewDataBlobFromBase64(text string) dataBlob {
	data, err := base64.StdEncoding.DecodeString(text)

	if err != nil {
		panic("decoding of text failed")
	}

	return dataBlob{
		Data: data,
	}
}

// Blob interface implementation
func (b dataBlob) GetSize() int64 {
	return int64(len(b.Data))
}

func (b dataBlob) GetMediaType() string {
	if b.MediaType != "" {
		return b.MediaType
	}

	return manifest.DockerV2Schema2LayerMediaType
}

func (b dataBlob) GetDigest() digest.Digest {
	return digest.FromBytes(b.Data)
}

func (b dataBlob) Reader() io.Reader {
	return bytes.NewReader(b.Data)
}

func MakeDescriptorForBlob(b Blob) manifest.Schema2Descriptor {
	return manifest.Schema2Descriptor{
		MediaType: b.GetMediaType(),
		Size:      b.GetSize(),
		Digest:    b.GetDigest(),
	}
}

// Repo //
type Repo struct {
	blobs     map[string]Blob
	manifests map[string]*manifest.Schema2
	images    map[string]*manifest.Schema2List
	tags      map[string]string
}

func NewRepo() *Repo {
	return &Repo{
		blobs:     make(map[string]Blob),
		manifests: make(map[string]*manifest.Schema2),
		tags:      make(map[string]string),
		images:    make(map[string]*manifest.Schema2List),
	}
}

func (r *Repo) AddBlob(b Blob) manifest.Schema2Descriptor {
	desc := MakeDescriptorForBlob(b)
	r.blobs[desc.Digest.String()] = b
	return desc
}

func (r *Repo) AddObject(v interface{}, mediaType string) manifest.Schema2Descriptor {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic("could not marshal image object")
	}

	blob := dataBlob{
		Data:      data,
		MediaType: mediaType,
	}

	return r.AddBlob(blob)
}

func (r *Repo) AddManifest(mf *manifest.Schema2) manifest.Schema2Descriptor {
	desc := r.AddObject(mf, mf.MediaType)

	r.manifests[desc.Digest.String()] = mf

	return desc
}

func (r *Repo) AddImage(layers []Blob, arches []string, comment string, ctime time.Time) string {

	blobs := make([]manifest.Schema2Descriptor, len(layers))

	for i, layer := range layers {
		blobs[i] = r.AddBlob(layer)
	}

	manifests := make([]manifest.Schema2ManifestDescriptor, len(arches))

	for i, arch := range arches {
		img := manifest.Schema2V1Image{
			Architecture: arch,
			OS:           "linux",
			Author:       "osbuild",
			Comment:      comment,
			Created:      ctime,
		}

		// Add the config object
		config := r.AddObject(img, manifest.DockerV2Schema2ConfigMediaType)

		// make and add the manifest object
		schema := manifest.Schema2FromComponents(config, blobs)
		mf := r.AddManifest(schema)

		desc := manifest.Schema2ManifestDescriptor{
			Schema2Descriptor: mf,
			Platform: manifest.Schema2PlatformSpec{
				Architecture: arch,
				OS:           "linux",
			},
		}

		manifests[i] = desc
	}

	list := manifest.Schema2ListFromComponents(manifests)
	desc := r.AddObject(list, list.MediaType)
	checksum := desc.Digest.String()

	r.images[checksum] = list
	r.tags["latest"] = checksum

	return checksum
}

func (r *Repo) AddTag(checksum, tag string) {

	if _, ok := r.images[checksum]; !ok {
		panic("cannot tag: image not found: " + checksum)
	}

	r.tags[tag] = checksum
}

func WriteBlob(blob Blob, w http.ResponseWriter) {
	w.Header().Add("Content-Type", blob.GetMediaType())
	w.Header().Add("Content-Length", fmt.Sprintf("%d", blob.GetSize()))
	w.Header().Add("Docker-Content-Digest", blob.GetDigest().String())
	w.WriteHeader(http.StatusOK)

	reader := blob.Reader()

	_, err := io.Copy(w, reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing blob: %v", err)
	}
}

func BlobIsManifest(blob Blob) bool {
	mt := blob.GetMediaType()
	return mt == manifest.DockerV2Schema2MediaType || mt == manifest.DockerV2ListMediaType
}

func (r *Repo) ServeManifest(ref string, w http.ResponseWriter, req *http.Request) {
	if checksum, ok := r.tags[ref]; ok {
		ref = checksum
	}

	blob, ok := r.blobs[ref]
	if !ok || !BlobIsManifest(blob) {
		fmt.Fprintf(os.Stderr, "manifest %s not found", ref)
		http.NotFound(w, req)
		return
	}

	WriteBlob(blob, w)
}

func (r *Repo) ServeBlob(ref string, w http.ResponseWriter, req *http.Request) {

	blob, ok := r.blobs[ref]

	if !ok {
		fmt.Fprintf(os.Stderr, "blob %s not found", ref)
		http.NotFound(w, req)
		return
	}

	WriteBlob(blob, w)
}

// Registry //

type Registry struct {
	server *httptest.Server
	repos  map[string]*Repo
}

func (reg *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	parts := strings.SplitN(req.URL.Path, "?", 1)
	paths := strings.Split(strings.Trim(parts[0], "/"), "/")

	// Possbile routes
	// [1] version-check:  /v2/
	// [2] blobs:          /v2/<repo_name>/blobs/<digest>
	// [3] manifest:       /v2/<repo_name>/manifests/<ref>
	//
	// we need at least 4 path components and path has to start with "/v2"

	if len(paths) < 1 || paths[0] != "v2" {
		http.NotFound(w, req)
		return
	}

	// [1] version check
	if len(paths) == 1 {
		w.WriteHeader(200)
		return
	} else if len(paths) < 4 {
		http.NotFound(w, req)
		return
	}

	// we asserted that we have at least 4 path components
	ref := paths[len(paths)-1]
	cmd := paths[len(paths)-2]

	repoName := strings.Join(paths[1:len(paths)-2], "/")

	repo, ok := reg.repos[repoName]
	if !ok {
		fmt.Fprintf(os.Stderr, "repo %s not found", repoName)
		http.NotFound(w, req)
		return
	}

	if cmd == "manifests" {
		repo.ServeManifest(ref, w, req)
	} else if cmd == "blobs" {
		repo.ServeBlob(ref, w, req)
	} else {
		http.NotFound(w, req)
	}
}

func NewTestRegistry() *Registry {

	reg := &Registry{
		repos: make(map[string]*Repo),
	}
	reg.server = httptest.NewTLSServer(reg)

	return reg
}

func (reg *Registry) AddRepo(name string) *Repo {
	repo := NewRepo()
	reg.repos[name] = repo
	return repo
}

func (reg *Registry) GetRef(repo string) string {
	return fmt.Sprintf("%s/%s", reg.server.Listener.Addr().String(), repo)
}

func (reg *Registry) Resolve(target, arch string) (container.Spec, error) {

	ref, err := reference.ParseNormalizedNamed(target)
	if err != nil {
		return container.Spec{}, fmt.Errorf("failed to parse '%s': %w", target, err)
	}

	domain := reference.Domain(ref)

	tag := "latest"
	var checksum string

	if tagged, ok := ref.(reference.NamedTagged); ok {
		tag = tagged.Tag()
	}

	if digested, ok := ref.(reference.Digested); ok {
		checksum = string(digested.Digest())
	}

	if domain != reg.server.Listener.Addr().String() {
		return container.Spec{}, fmt.Errorf("unknown domain")
	}

	ref = reference.TrimNamed(ref)
	path := reference.Path(ref)

	repo, ok := reg.repos[path]
	if !ok {
		return container.Spec{}, fmt.Errorf("unknown repo")
	}

	if checksum == "" {
		checksum, ok = repo.tags[tag]
		if !ok {
			return container.Spec{}, fmt.Errorf("unknown tag")
		}
	}

	lst, ok := repo.images[checksum]

	if ok {
		checksum = ""

		for _, m := range lst.Manifests {
			if m.Platform.Architecture == arch {
				checksum = m.Digest.String()
				break
			}
		}

		if checksum == "" {
			return container.Spec{}, fmt.Errorf("unsupported architecture")
		}
	}

	mf, ok := repo.manifests[checksum]
	if !ok {
		return container.Spec{}, fmt.Errorf("unknown digest")
	}

	return container.Spec{
		Source:    ref.String(),
		Digest:    checksum,
		ImageID:   mf.ConfigDescriptor.Digest.String(),
		LocalName: ref.String(),
		TLSVerify: common.BoolToPtr(false),
	}, nil
}

func (reg *Registry) Close() {
	reg.server.Close()
}
