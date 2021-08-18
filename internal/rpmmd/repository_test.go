package rpmmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_CurlProxyString(t *testing.T) {
	var cases = []struct {
		Pkg      PackageSpec
		Expected string
	}{{
		PackageSpec{
			Name:    "dep-package1",
			Epoch:   0,
			Version: "1.33",
			Release: "2.fc30",
			Arch:    "x86_64",
		},
		"",
	}, {
		PackageSpec{
			Name:    "dep-package2",
			Epoch:   0,
			Version: "1.42",
			Release: "0.fc30",
			Arch:    "x86_64",
			Proxy:   "http://proxy.host.com:8123",
		},
		"http://proxy.host.com:8123",
	}, {
		PackageSpec{
			Name:          "dep-package3",
			Epoch:         0,
			Version:       "1.42",
			Release:       "0.fc30",
			Arch:          "x86_64",
			Proxy:         "http://proxy.host.com:8123",
			ProxyUsername: "whistler",
		},
		"http://whistler:@proxy.host.com:8123",
	}, {
		PackageSpec{
			Name:          "dep-package4",
			Epoch:         0,
			Version:       "1.42",
			Release:       "0.fc30",
			Arch:          "x86_64",
			Proxy:         "http://proxy.host.com:8123",
			ProxyUsername: "whistler",
			ProxyPassword: "setecastronomy",
		},
		"http://whistler:setecastronomy@proxy.host.com:8123",
	}, {
		PackageSpec{
			Name:          "dep-package5",
			Epoch:         0,
			Version:       "1.42",
			Release:       "0.fc30",
			Arch:          "x86_64",
			Proxy:         "http://proxy.host.com:8123",
			ProxyPassword: "setecastronomy",
		},
		"http://:setecastronomy@proxy.host.com:8123",
	},
	}

	for _, c := range cases {
		assert.Equal(t, c.Expected, c.Pkg.CurlProxyString())
	}
}
