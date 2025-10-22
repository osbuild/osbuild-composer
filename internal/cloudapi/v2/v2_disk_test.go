package v2_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/test"
)

func TestComposeDiskCustomizationsValidation(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false, false)
	defer cancel()

	testCases := map[string]string{
		"simple": `
{
	"disk": {
		"minsize": "100 GiB",
		"partitions": [
			{
				"mountpoint": "/home",
				"fs_type": "ext4",
				"minsize": "2 GiB"
			}
		]
	}
}
`,

		"simple-with-type": `
{
	"disk": {
		"minsize": "100 GiB",
		"partitions": [
			{
				"type": "plain",
				"mountpoint": "/home",
				"fs_type": "xfs",
				"minsize": "2 GiB"
			}
		]
	}
}
`,

		"large-plain": `
{
	"disk": {
		"type": "gpt",
		"partitions": [
			{
				"mountpoint": "/data",
				"fs_type": "ext4",
				"minsize": "1 GiB"
			},
			{
				"mountpoint": "/home",
				"label": "home",
				"fs_type": "ext4",
				"minsize": "2 GiB"
			},
			{
				"mountpoint": "/home/shadowman",
				"fs_type": "ext4",
				"minsize": "5 GiB"
			},
			{
				"mountpoint": "/var",
				"fs_type": "xfs",
				"minsize": "4 GiB"
			},
			{
				"mountpoint": "/opt",
				"fs_type": "ext4",
				"minsize": "10 GiB"
			},
			{
				"mountpoint": "/media",
				"fs_type": "ext4",
				"minsize": "9 GiB"
			},
			{
				"mountpoint": "/root",
				"fs_type": "ext4",
				"minsize": "1 GiB"
			},
			{
				"mountpoint": "/srv",
				"fs_type": "xfs",
				"minsize": "3 GiB"
			},
			{
				"fs_type": "swap",
				"minsize": "1 GiB"
			}
		]
	}
}
`,

		"lvm": `
{
	"disk": {
		"type": "gpt",
		"partitions": [
			{
				"mountpoint": "/data",
				"minsize": "1 GiB",
				"label": "data",
				"fs_type": "ext4"
			},
			{
				"type": "lvm",
				"name": "testvg",
				"minsize": "200 GiB",
				"logical_volumes": [
					{
						"name": "homelv",
						"mountpoint": "/home",
						"label": "home",
						"fs_type": "ext4",
						"minsize": "2 GiB"
					},
					{
						"name": "shadowmanlv",
						"mountpoint": "/home/shadowman",
						"fs_type": "ext4",
						"minsize": "5 GiB"
					},
					{
						"name": "foolv",
						"mountpoint": "/foo",
						"fs_type": "ext4",
						"minsize": "1 GiB"
					},
					{
						"name": "usrlv",
						"mountpoint": "/usr",
						"fs_type": "ext4",
						"minsize": "4 GiB"
					},
					{
						"name": "optlv",
						"mountpoint": "/opt",
						"fs_type": "ext4",
						"minsize": "1 GiB"
					},
					{
						"name": "medialv",
						"mountpoint": "/media",
						"fs_type": "ext4",
						"minsize": "10 GiB"
					},
					{
						"name": "roothomelv",
						"mountpoint": "/root",
						"fs_type": "ext4",
						"minsize": "1 GiB"
					},
					{
						"name": "srvlv",
						"mountpoint": "/srv",
						"fs_type": "ext4",
						"minsize": "10 GiB"
					},
					{
						"name": "swap-lv",
						"fs_type": "swap",
						"minsize": "1 GiB"
					}
				]
			}
		]
	}
}
`,
		"btrfs": `
{
	"disk": {
		"partitions": [
			{
				"type": "plain",
				"mountpoint": "/data",
				"minsize": "12 GiB",
				"fs_type": "xfs"
			},
			{
				"type": "btrfs",
				"minsize": "10 GiB",
				"subvolumes": [
					{
						"name": "subvol-home",
						"mountpoint": "/home"
					},
					{
						"name": "subvol-shadowman",
						"mountpoint": "/home/shadowman"
					},
					{
						"name": "subvol-foo",
						"mountpoint": "/foo"
					},
					{
						"name": "subvol-usr",
						"mountpoint": "/usr"
					},
					{
						"name": "subvol-opt",
						"mountpoint": "/opt"
					},
					{
						"name": "subvol-media",
						"mountpoint": "/media"
					},
					{
						"name": "subvol-root",
						"mountpoint": "/root"
					},
					{
						"name": "subvol-srv",
						"mountpoint": "/srv"
					}
				]
			},
			{
				"type": "plain",
				"fs_type": "swap",
				"label": "swap-part",
				"minsize": "1 GiB"
			}
		]
	}
}
`,
	}

	resp := `{"href": "/api/image-builder-composer/v2/compose", "kind": "ComposeId"}`
	for name, customizations := range testCases {
		t.Run(name, func(t *testing.T) {
			body := fmt.Sprintf(`
	{
		"distribution": "%s",
		"customizations": %s,
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		}
	}`, test_distro.TestDistro1Name, customizations, test_distro.TestArch3Name)

			test.TestRouteWithReply(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", body, http.StatusCreated, resp, "id")
		})
	}
}

func TestComposeDiskCustomizationsErrors(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false, false)
	defer cancel()

	type testCase struct {
		customizations string
		expResponse    string
	}

	// one of two responses is returned depending on the exact origin of the error:
	//  1. IMAGE-BUILDER-COMPOSER-30 is returned when the customization failed at the schema level, when it fails to validate against the openapi schema
	//  2. IMAGE-BUILDER-COMPOSER-35 is returned when the customization failed the Disk.Validate() call at the blueprint level
	validationErrorResp := `{"href": "/api/image-builder-composer/v2/errors/30", "kind": "Error", "reason": "Request could not be validated", "code": "IMAGE-BUILDER-COMPOSER-30"}`
	invalidCustomizationResp := `{"href": "/api/image-builder-composer/v2/errors/35", "kind": "Error", "reason": "Invalid image customization", "code": "IMAGE-BUILDER-COMPOSER-35"}`

	testCases := map[string]testCase{
		"empty": {
			customizations: `{"disk": {}}`,
			expResponse:    validationErrorResp,
		},
		"number-minsize": {
			customizations: `
{
	"disk": {
		"minsize": 1024,
		"partitions": [
			{
				"mountpoint": "/",
				"minsize": "2 GiB",
				"fs_type": "ext4"
			}
		]
	}
}
`,
			expResponse: validationErrorResp,
		},
		"no-fs-type": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"mountpoint": "/",
				"minsize": "2 GiB"
			}
		]
	}
}
`,
			expResponse: validationErrorResp,
		},
		"swap-with-mountpoint": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"mountpoint": "/",
				"fs_type": "swap"
			}
		]
	}
}
`,
			expResponse: invalidCustomizationResp,
		},
		"nonswap-without-mountpoint": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"fs_type": "xfs"
			}
		]
	}
}
`,
			expResponse: invalidCustomizationResp,
		},
		"swap-with-mountpoint-lvm": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"type": "lvm",
				"logical_volumes": [{
					"mountpoint": "/",
					"fs_type": "swap"
				}]
			}
		]
	}
}
`,
			expResponse: invalidCustomizationResp,
		},
		"nonswap-without-mountpoint-lvm": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"type": "lvm",
				"logical_volumes": [{
					"fs_type": "xfs"
				}]
			}
		]
	}
}
`,
			expResponse: invalidCustomizationResp,
		},
		"notype-with-lv": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"logical_volumes": [
					{
						"name": "homelv",
						"mountpoint": "/home",
						"label": "home",
						"fs_type": "ext4",
						"minsize": "2 GiB"
					}
				]
			}
		]
	}
}
`,
			expResponse: validationErrorResp,
		},
		"plain-with-lv": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"type": "plain",
				"logical_volumes": [
					{
						"name": "homelv",
						"mountpoint": "/home",
						"label": "home",
						"fs_type": "ext4",
						"minsize": "2 GiB"
					}
				]
			}
		]
	}
}
`,
			expResponse: validationErrorResp,
		},
		"notype-with-subvol": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"subvolumes": [
					{
						"name": "home",
						"mountpoint": "/home"
					}
				]
			}
		]
	}
}
`,
			expResponse: validationErrorResp,
		},
		"plain-with-subvol": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"type": "plain",
				"subvolumes": [
					{
						"name": "home",
						"mountpoint": "/home"
					}
				]
			}
		]
	}
}
`,
			expResponse: validationErrorResp,
		},
		"lvm-with-subvol": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"type": "lvm",
				"subvolumes": [
					{
						"name": "home",
						"mountpoint": "/home"
					}
				]
			}
		]
	}
}
`,
			expResponse: validationErrorResp,
		},
		"btrfs-with-lv": {
			customizations: `
{
	"disk": {
		"partitions": [
			{
				"type": "btrfs",
				"logical_volumes": [
					{
						"name": "homelv",
						"mountpoint": "/home",
						"label": "home",
						"fs_type": "ext4",
						"minsize": "2 GiB"
					}
				]
			}
		]
	}
}
`,
			expResponse: validationErrorResp,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			body := fmt.Sprintf(`
	{
		"distribution": "%s",
		"customizations": %s,
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		}
	}`, test_distro.TestDistro1Name, tc.customizations, test_distro.TestArch3Name)
			test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", body, http.StatusBadRequest, tc.expResponse, "id", "operation_id", "details")
		})
	}
}
