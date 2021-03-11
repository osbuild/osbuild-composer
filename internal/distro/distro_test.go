package distro_test

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora33"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel84"
)

func TestDistro_Manifest(t *testing.T) {
	distro_test_common.TestDistro_Manifest(
		t,
		"../../test/data/manifests/",
		"*",
		fedora32.New(), fedora33.New(), rhel8.New(), rhel84.New(), rhel84.NewCentos(),
	)
}
