package osbuild

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewSkopeoSource(t *testing.T) {
	testDigest := "sha256:f29b6cd42a94a574583439addcd6694e6224f0e4b32044c9e3aee4c4856c2a50"
	imageID := "sha256:c2ecf25cf190e76b12b07436ad5140d4ba53d8a136d498705e57a006837a720f"

	source := NewSkopeoSource()

	source.AddItem("name", testDigest, imageID, common.BoolToPtr(false))
	assert.Len(t, source.Items, 1)

	item, ok := source.Items[imageID]
	assert.True(t, ok)
	assert.Equal(t, item.Image.Name, "name")
	assert.Equal(t, item.Image.Digest, testDigest)
	assert.Equal(t, item.Image.TLSVerify, common.BoolToPtr(false))

	testDigest = "sha256:d49eebefb6c7ce5505594bef652bd4adc36f413861bd44209d9b9486310b1264"
	imageID = "sha256:d2ab8fea7f08a22f03b30c13c6ea443121f25e87202a7496e93736efa6fe345a"

	source.AddItem("name2", testDigest, imageID, nil)
	assert.Len(t, source.Items, 2)
	item, ok = source.Items[imageID]
	assert.True(t, ok)
	assert.Nil(t, item.Image.TLSVerify)

	// empty name
	assert.Panics(t, func() {
		source.AddItem("", testDigest, imageID, nil)
	})

	// empty digest
	assert.Panics(t, func() {
		source.AddItem("name", "", imageID, nil)
	})

	// empty image id
	assert.Panics(t, func() {
		source.AddItem("name", testDigest, "", nil)
	})

	// invalid digest
	assert.Panics(t, func() {
		source.AddItem("name", "foo", imageID, nil)
	})

	// invalid image id
	assert.Panics(t, func() {
		source.AddItem("name", testDigest, "sha256:foo", nil)
	})
}
