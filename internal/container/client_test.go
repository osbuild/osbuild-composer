package container_test

import (
	"context"
	"testing"
	"time"

	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/stretchr/testify/assert"
)

//

func TestClientResolve(t *testing.T) {

	registry := NewTestRegistry()
	defer registry.Close()

	repo := registry.AddRepo("library/osbuild")
	repo.AddImage(
		[]Blob{NewDataBlobFromBase64(rootLayer)},
		[]string{"amd64", "ppc64le"},
		"cool container",
		time.Time{})

	ref := registry.GetRef("library/osbuild")
	client, err := container.NewClient(ref)

	assert.NoError(t, err)
	assert.NotNil(t, client)

	client.SkipTLSVerify()

	ctx := context.Background()

	client.SetArchitectureChoice("amd64")
	spec, err := client.Resolve(ctx, "")

	assert.NoError(t, err)
	assert.Equal(t, container.Spec{
		Source:    ref,
		Digest:    "sha256:f29b6cd42a94a574583439addcd6694e6224f0e4b32044c9e3aee4c4856c2a50",
		ImageID:   "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f",
		TLSVerify: client.GetTLSVerify(),
		LocalName: ref,
	}, spec)

	client.SetArchitectureChoice("ppc64le")
	spec, err = client.Resolve(ctx, "")

	assert.NoError(t, err)
	assert.Equal(t, container.Spec{
		Source:    ref,
		Digest:    "sha256:d49eebefb6c7ce5505594bef652bd4adc36f413861bd44209d9b9486310b1264",
		ImageID:   "sha256:d2ab8fea7f08a22f03b30c13c6ea443121f25e87202a7496e93736efa6fe345a",
		TLSVerify: client.GetTLSVerify(),
		LocalName: ref,
	}, spec)

	// don't have that architecture
	client.SetArchitectureChoice("s390x")
	_, err = client.Resolve(ctx, "")

	assert.Error(t, err)
}
