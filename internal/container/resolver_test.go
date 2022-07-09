package container_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/stretchr/testify/assert"
)

type lessCompare func(i, j int) bool

func makeSpecSorter(specs []container.Spec) lessCompare {
	return func(i, j int) bool {
		return specs[i].Digest < specs[j].Digest
	}
}

func TestResolver(t *testing.T) {

	registry := NewTestRegistry()
	defer registry.Close()

	repo := registry.AddRepo("library/osbuild")
	ref := registry.GetRef("library/osbuild")

	refs := make([]string, 10)
	for i := 0; i < len(refs); i++ {
		checksum := repo.AddImage(
			[]Blob{NewDataBlobFromBase64(rootLayer)},
			[]string{"amd64", "ppc64le"},
			fmt.Sprintf("image %d", i),
			time.Time{})

		tag := fmt.Sprintf("%d", i)
		repo.AddTag(checksum, tag)
		refs[i] = fmt.Sprintf("%s:%s", ref, tag)
	}

	resolver := container.NewResolver("amd64")

	for _, r := range refs {
		resolver.Add(r, "", common.BoolToPtr(false))
	}

	have, err := resolver.Finish()
	assert.NoError(t, err)
	assert.NotNil(t, have)

	assert.Len(t, have, len(refs))

	want := make([]container.Spec, len(refs))
	for i, r := range refs {
		spec, err := registry.Resolve(r, "amd64")
		assert.NoError(t, err)
		want[i] = spec
	}

	sort.Slice(have, makeSpecSorter(have))
	sort.Slice(want, makeSpecSorter(want))

	assert.ElementsMatch(t, have, want)
}

func TestResolverFail(t *testing.T) {
	resolver := container.NewResolver("amd64")

	resolver.Add("invalid-reference@${IMAGE_DIGEST}", "", common.BoolToPtr(false))

	specs, err := resolver.Finish()
	assert.Error(t, err)
	assert.Len(t, specs, 0)

	registry := NewTestRegistry()
	defer registry.Close()

	resolver.Add(registry.GetRef("repo"), "", common.BoolToPtr(false))
	specs, err = resolver.Finish()
	assert.Error(t, err)
	assert.Len(t, specs, 0)
}
