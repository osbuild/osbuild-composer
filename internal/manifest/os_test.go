package manifest

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewTestOS returns a minimally populated OS struct for use in testing
func NewTestOS() *OS {
	repos := []rpmmd.RepoConfig{}
	manifest := New()
	runner := &runner.Fedora{Version: 36}
	build := NewBuild(&manifest, runner, repos)
	build.Checkpoint()

	// create an x86_64 platform with bios boot
	platform := &platform.X86{
		BIOS: true,
	}

	os := NewOS(&manifest, build, platform, repos)
	packages := []rpmmd.PackageSpec{
		rpmmd.PackageSpec{Name: "pkg1"},
	}
	os.serializeStart(packages)

	return os
}

// CheckFirstBootStageOptions checks the Command strings
func CheckFirstBootStageOptions(t *testing.T, stages []*osbuild.Stage, commands []string) {
	// Find the FirstBootStage
	for _, s := range stages {
		if s.Type == "org.osbuild.first-boot" {
			require.NotNil(t, s.Options)
			options, ok := s.Options.(*osbuild.FirstBootStageOptions)
			require.True(t, ok)
			require.Equal(t, len(options.Commands), len(commands))

			// Make sure the commands are the same
			for idx, cmd := range commands {
				assert.Equal(t, cmd, options.Commands[idx])
			}
		}
	}
}

// CheckPkgSetInclude makes sure the packages named in pkgs are all included
func CheckPkgSetInclude(t *testing.T, pkgSetChain []rpmmd.PackageSet, pkgs []string) {

	// Gather up all the includes
	var includes []string
	for _, ps := range pkgSetChain {
		includes = append(includes, ps.Include...)
	}

	for _, p := range pkgs {
		assert.Contains(t, includes, p)
	}
}

func TestSubscriptionManagerCommands(t *testing.T) {
	os := NewTestOS()
	os.Subscription = &distro.SubscriptionImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
	}
	pipeline := os.serialize()
	CheckFirstBootStageOptions(t, pipeline.Stages, []string{
		"/usr/sbin/subscription-manager register --org=2040324 --activationkey=my-secret-key --serverurl subscription.rhsm.redhat.com --baseurl http://cdn.redhat.com/",
	})
}

func TestSubscriptionManagerInsightsCommands(t *testing.T) {
	os := NewTestOS()
	os.Subscription = &distro.SubscriptionImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      true,
	}
	pipeline := os.serialize()
	CheckFirstBootStageOptions(t, pipeline.Stages, []string{
		"/usr/sbin/subscription-manager register --org=2040324 --activationkey=my-secret-key --serverurl subscription.rhsm.redhat.com --baseurl http://cdn.redhat.com/",
		"/usr/bin/insights-client --register",
		"restorecon -R /root/.gnupg",
	})
}

func TestRhcInsightsCommands(t *testing.T) {
	os := NewTestOS()
	os.Subscription = &distro.SubscriptionImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      false,
		Rhc:           true,
	}
	pipeline := os.serialize()
	CheckFirstBootStageOptions(t, pipeline.Stages, []string{
		"/usr/bin/rhc connect -o=2040324 -a=my-secret-key --server subscription.rhsm.redhat.com",
		"restorecon -R /root/.gnupg",
		"/usr/sbin/semanage permissive --add rhcd_t",
	})
}

func TestSubscriptionManagerPackages(t *testing.T) {
	os := NewTestOS()
	os.Subscription = &distro.SubscriptionImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
	}

	CheckPkgSetInclude(t, os.getPackageSetChain(), []string{"subscription-manager"})
}

func TestSubscriptionManagerInsightsPackages(t *testing.T) {
	os := NewTestOS()
	os.Subscription = &distro.SubscriptionImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      true,
	}
	CheckPkgSetInclude(t, os.getPackageSetChain(), []string{"subscription-manager", "insights-client"})
}

func TestRhcInsightsPackages(t *testing.T) {
	os := NewTestOS()
	os.Subscription = &distro.SubscriptionImageOptions{
		Organization:  "2040324",
		ActivationKey: "my-secret-key",
		ServerUrl:     "subscription.rhsm.redhat.com",
		BaseUrl:       "http://cdn.redhat.com/",
		Insights:      false,
		Rhc:           true,
	}
	CheckPkgSetInclude(t, os.getPackageSetChain(), []string{"rhc", "subscription-manager", "insights-client"})
}
