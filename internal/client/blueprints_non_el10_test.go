//go:build !rhel10

package client

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When gsl with version * was specified in the blueprint,
// composer depsolved both x86_64 and i686 version of gsl.
// This test case should prevent this from happening.
// gsl is used because it has x86_64 and i686 versions on both RHEL and Fedora.
// Also, gsl-devel package exists, which is not dependant on gsl and shouldn't
// be depsolved.
//
// NB: This test is skipped on RHEL 10 and CentOS 10 because there are no
// i686 packages available in the repositories.
func TestMultilibBlueprintDepsolveV0(t *testing.T) {
	if testState.unitTest {
		t.Skip()
	}
	versionStrings := []string{"*", "2.*", ""}
	for _, versionString := range versionStrings {
		t.Run(versionString, func(t *testing.T) {
			bp := `{
				"name": "test-multilib-deps-blueprint-v0",
				"description": "CheckBlueprintDepsolveV0",
				"version": "0.0.1",
				"packages": [{"name": "gsl", "version": "` + versionString + `"}]
			}`

			resp, err := PostJSONBlueprintV0(testState.socket, bp)
			require.NoError(t, err, "POST blueprint failed with a client error")
			require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

			deps, api, err := DepsolveBlueprintV0(testState.socket, "test-multilib-deps-blueprint-v0")
			require.NoError(t, err, "Depsolve blueprint failed with a client error")
			require.Nil(t, api, "DepsolveBlueprint failed: %#v", api)

			gslCount := 0
			for _, dep := range deps.Blueprints[0].Dependencies {
				if strings.HasPrefix(dep.Name, "gsl") {
					gslCount += 1
				}
			}

			if !assert.Equalf(t, 1, gslCount, "gsl is specified %d-times in the depsolve, should be there only once", gslCount) {
				depsolveOutput, err := json.MarshalIndent(deps, "", "  ")
				require.NoError(t, err)
				t.Logf("depsolve output:\n%s", depsolveOutput)
				t.FailNow()
			}
		})
	}
}
