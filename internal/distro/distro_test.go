package distro_test

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
)

func TestDistro_Manifest(t *testing.T) {

	distro_test_common.TestDistro_Manifest(
		t,
		"../../test/data/manifests/",
		"*",
		distroregistry.NewDefault(),
	)
}
