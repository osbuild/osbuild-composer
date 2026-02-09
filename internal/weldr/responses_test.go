package weldr

import (
	"fmt"

	depsolvednf_mock "github.com/osbuild/osbuild-composer/internal/mocks/depsolvednf"
)

// Expected responses for API tests
//
// Smaller responses can be defined in-line, but larger, complex structures
// should be defined here, either as raw strings, or as structured objects.
// Raw strings should also be formatted to make reading and diffing easier.

const freezeTestResponse = `
{
  "blueprints": [
    {
      "blueprint": {
        "name": "test",
        "description": "Test",
        "version": "0.0.1",
        "packages": [
          {
            "name": "dep-package1",
            "version": "1.33-2.fc30.x86_64"
          },
          {
            "name": "dep-package3",
            "version": "7:3.0.3-1.fc30.x86_64"
          }
        ],
        "modules": [
          {
            "name": "dep-package2",
            "version": "2.9-1.fc30.x86_64"
          }
        ],
        "groups": [],
        "enabled_modules": []
      }
    },
    {
      "blueprint": {
        "name": "test2",
        "description": "Test",
        "version": "0.0.0",
        "packages": [
          {
            "name": "dep-package1",
            "version": "1.33-2.fc30.x86_64"
          },
          {
            "name": "dep-package3",
            "version": "7:3.0.3-1.fc30.x86_64"
          }
        ],
        "modules": [
          {
            "name": "dep-package2",
            "version": "2.9-1.fc30.x86_64"
          }
        ],
        "groups": [],
        "enabled_modules": []
      }
    }
  ],
  "errors": []
}
`

func depsolveDependenciesPartialResponse(repoID string) string {
	return fmt.Sprintf(`[
        {
          "name": "dep-package1",
          "epoch": 0,
          "version": "1.33",
          "release": "2.fc30",
          "remote_location": "https://pkg1.example.com/1.33-2.fc30.x86_64.rpm",
          "repo_id": "%[1]s",
          "arch": "x86_64",
          "check_gpg": true,
          "checksum": "sha256:fe3951d112c3b1c84dc8eac57afe0830df72df1ca0096b842f4db5d781189893"
        },
        {
          "name": "dep-package2",
          "epoch": 0,
          "version": "2.9",
          "release": "1.fc30",
          "remote_location": "https://pkg2.example.com/2.9-1.fc30.x86_64.rpm",
          "repo_id": "%[1]s",
          "arch": "x86_64",
          "check_gpg": true,
          "checksum": "sha256:5797c0b0489681596b5b3cd7165d49870b85b69d65e08770946380a3dcd49ea2"
        },
        {
          "name": "dep-package3",
          "epoch": 7,
          "version": "3.0.3",
          "release": "1.fc30",
          "remote_location": "https://pkg3.example.com/3.0.3-1.fc30.x86_64.rpm",
          "repo_id": "%[1]s",
          "arch": "x86_64",
          "check_gpg": true,
          "checksum": "sha256:62278d360aa5045eb202af39fe85743a4b5615f0c9c7439a04d75d785db4c720"
        }
      ]
`, repoID)
}

var depsolveTestResponse = fmt.Sprintf(`
{
  "blueprints": [
    {
      "blueprint": {
        "name": "test",
        "description": "Test",
        "version": "0.0.1",
        "packages": [
          {
            "name": "dep-package1",
            "version": "*"
          }
        ],
        "enabled_modules": [],
        "groups": [],
        "modules": [
          {
            "name": "dep-package3",
            "version": "*"
          }
        ]
      },
      "dependencies": %s
    }
  ],
  "errors": []
}
`, depsolveDependenciesPartialResponse(testRepoID))

var depsolvePackageNotExistErrorAPIResponse = fmt.Sprintf(`
{
  "blueprints": [
    {
      "blueprint": {
        "name": "test",
        "description": "Test",
        "version": "0.0.1",
        "packages": [
          {
            "name": "dep-package1",
            "version": "*"
          }
        ],
        "groups": [],
        "modules": [
          {
            "name": "dep-package3",
            "version": "*"
          }
        ],
        "enabled_modules": []
      },
      "dependencies": []
    }
  ],
  "errors": [
    {
      "id": "BlueprintsError",
      "msg": %q
    }
  ]
}
`, "test: running osbuild-depsolve-dnf failed:\n"+depsolvednf_mock.DepsolvePackageNotExistError.Error())

var depsolveBadErrorAPIResponse = fmt.Sprintf(`
{
  "blueprints": [
    {
      "blueprint": {
        "name": "test",
        "description": "Test",
        "version": "0.0.1",
        "packages": [
          {
            "name": "dep-package1",
            "version": "*"
          }
        ],
        "groups": [],
        "modules": [
          {
            "name": "dep-package3",
            "version": "*"
          }
        ],
        "enabled_modules": []
      },
      "dependencies": []
    }
  ],
  "errors": [
    {
      "id": "BlueprintsError",
      "msg": %q
    }
  ]
}
`, "test: running osbuild-depsolve-dnf failed:\n"+depsolvednf_mock.DepsolveBadError.Error())

const oldBlueprintsUndoResponse = `
{
  "blueprints": [
    {
      "changes": [
        {
          "commit": "",
          "message": "Change tmux version",
          "revision": null,
          "timestamp": ""
        },
        {
          "commit": "",
          "message": "Add tmux package",
          "revision": null,
          "timestamp": ""
        },
        {
          "commit": "",
          "message": "Initial commit",
          "revision": null,
          "timestamp": ""
        }
      ],
      "name": "test-old-changes",
      "total": 3
    }
  ],
  "errors": [],
  "limit": 20,
  "offset": 0
}
`

const projectsInfoResponse = `
{
  "projects": [
    {
      "name": "package0",
      "summary": "pkg0 sum",
      "description": "pkg0 desc",
      "homepage": "https://pkg0.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-01-02T15:04:05",
          "epoch": 0,
          "release": "0.fc30",
          "source": {
            "license": "MIT",
            "version": "0.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-01-03T15:04:05",
          "epoch": 0,
          "release": "0.fc30",
          "source": {
            "license": "MIT",
            "version": "0.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package1",
      "summary": "pkg1 sum",
      "description": "pkg1 desc",
      "homepage": "https://pkg1.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-02-02T15:04:05",
          "epoch": 0,
          "release": "1.fc30",
          "source": {
            "license": "MIT",
            "version": "1.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-02-03T15:04:05",
          "epoch": 0,
          "release": "1.fc30",
          "source": {
            "license": "MIT",
            "version": "1.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package10",
      "summary": "pkg10 sum",
      "description": "pkg10 desc",
      "homepage": "https://pkg10.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-11-02T15:04:05",
          "epoch": 0,
          "release": "10.fc30",
          "source": {
            "license": "MIT",
            "version": "10.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-11-03T15:04:05",
          "epoch": 0,
          "release": "10.fc30",
          "source": {
            "license": "MIT",
            "version": "10.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package11",
      "summary": "pkg11 sum",
      "description": "pkg11 desc",
      "homepage": "https://pkg11.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-12-02T15:04:05",
          "epoch": 0,
          "release": "11.fc30",
          "source": {
            "license": "MIT",
            "version": "11.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-12-03T15:04:05",
          "epoch": 0,
          "release": "11.fc30",
          "source": {
            "license": "MIT",
            "version": "11.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package12",
      "summary": "pkg12 sum",
      "description": "pkg12 desc",
      "homepage": "https://pkg12.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-01-02T15:04:05",
          "epoch": 0,
          "release": "12.fc30",
          "source": {
            "license": "MIT",
            "version": "12.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-01-03T15:04:05",
          "epoch": 0,
          "release": "12.fc30",
          "source": {
            "license": "MIT",
            "version": "12.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package13",
      "summary": "pkg13 sum",
      "description": "pkg13 desc",
      "homepage": "https://pkg13.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-02-02T15:04:05",
          "epoch": 0,
          "release": "13.fc30",
          "source": {
            "license": "MIT",
            "version": "13.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-02-03T15:04:05",
          "epoch": 0,
          "release": "13.fc30",
          "source": {
            "license": "MIT",
            "version": "13.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package14",
      "summary": "pkg14 sum",
      "description": "pkg14 desc",
      "homepage": "https://pkg14.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-03-02T15:04:05",
          "epoch": 0,
          "release": "14.fc30",
          "source": {
            "license": "MIT",
            "version": "14.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-03-03T15:04:05",
          "epoch": 0,
          "release": "14.fc30",
          "source": {
            "license": "MIT",
            "version": "14.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package15",
      "summary": "pkg15 sum",
      "description": "pkg15 desc",
      "homepage": "https://pkg15.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-04-02T15:04:05",
          "epoch": 0,
          "release": "15.fc30",
          "source": {
            "license": "MIT",
            "version": "15.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-04-03T15:04:05",
          "epoch": 0,
          "release": "15.fc30",
          "source": {
            "license": "MIT",
            "version": "15.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package16",
      "summary": "pkg16 sum",
      "description": "pkg16 desc",
      "homepage": "https://pkg16.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-05-02T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-05-03T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package17",
      "summary": "pkg17 sum",
      "description": "pkg17 desc",
      "homepage": "https://pkg17.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-06-02T15:04:05",
          "epoch": 0,
          "release": "17.fc30",
          "source": {
            "license": "MIT",
            "version": "17.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-06-03T15:04:05",
          "epoch": 0,
          "release": "17.fc30",
          "source": {
            "license": "MIT",
            "version": "17.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package18",
      "summary": "pkg18 sum",
      "description": "pkg18 desc",
      "homepage": "https://pkg18.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-07-02T15:04:05",
          "epoch": 0,
          "release": "18.fc30",
          "source": {
            "license": "MIT",
            "version": "18.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-07-03T15:04:05",
          "epoch": 0,
          "release": "18.fc30",
          "source": {
            "license": "MIT",
            "version": "18.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package19",
      "summary": "pkg19 sum",
      "description": "pkg19 desc",
      "homepage": "https://pkg19.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-08-02T15:04:05",
          "epoch": 0,
          "release": "19.fc30",
          "source": {
            "license": "MIT",
            "version": "19.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-08-03T15:04:05",
          "epoch": 0,
          "release": "19.fc30",
          "source": {
            "license": "MIT",
            "version": "19.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package2",
      "summary": "pkg2 sum",
      "description": "pkg2 desc",
      "homepage": "https://pkg2.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-03-02T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-03-03T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package20",
      "summary": "pkg20 sum",
      "description": "pkg20 desc",
      "homepage": "https://pkg20.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-09-02T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-09-03T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package21",
      "summary": "pkg21 sum",
      "description": "pkg21 desc",
      "homepage": "https://pkg21.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-10-02T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-10-03T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package3",
      "summary": "pkg3 sum",
      "description": "pkg3 desc",
      "homepage": "https://pkg3.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-04-02T15:04:05",
          "epoch": 0,
          "release": "3.fc30",
          "source": {
            "license": "MIT",
            "version": "3.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-04-03T15:04:05",
          "epoch": 0,
          "release": "3.fc30",
          "source": {
            "license": "MIT",
            "version": "3.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package4",
      "summary": "pkg4 sum",
      "description": "pkg4 desc",
      "homepage": "https://pkg4.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-05-02T15:04:05",
          "epoch": 0,
          "release": "4.fc30",
          "source": {
            "license": "MIT",
            "version": "4.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-05-03T15:04:05",
          "epoch": 0,
          "release": "4.fc30",
          "source": {
            "license": "MIT",
            "version": "4.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package5",
      "summary": "pkg5 sum",
      "description": "pkg5 desc",
      "homepage": "https://pkg5.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-06-02T15:04:05",
          "epoch": 0,
          "release": "5.fc30",
          "source": {
            "license": "MIT",
            "version": "5.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-06-03T15:04:05",
          "epoch": 0,
          "release": "5.fc30",
          "source": {
            "license": "MIT",
            "version": "5.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package6",
      "summary": "pkg6 sum",
      "description": "pkg6 desc",
      "homepage": "https://pkg6.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-07-02T15:04:05",
          "epoch": 0,
          "release": "6.fc30",
          "source": {
            "license": "MIT",
            "version": "6.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-07-03T15:04:05",
          "epoch": 0,
          "release": "6.fc30",
          "source": {
            "license": "MIT",
            "version": "6.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package7",
      "summary": "pkg7 sum",
      "description": "pkg7 desc",
      "homepage": "https://pkg7.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-08-02T15:04:05",
          "epoch": 0,
          "release": "7.fc30",
          "source": {
            "license": "MIT",
            "version": "7.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-08-03T15:04:05",
          "epoch": 0,
          "release": "7.fc30",
          "source": {
            "license": "MIT",
            "version": "7.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package8",
      "summary": "pkg8 sum",
      "description": "pkg8 desc",
      "homepage": "https://pkg8.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-09-02T15:04:05",
          "epoch": 0,
          "release": "8.fc30",
          "source": {
            "license": "MIT",
            "version": "8.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-09-03T15:04:05",
          "epoch": 0,
          "release": "8.fc30",
          "source": {
            "license": "MIT",
            "version": "8.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package9",
      "summary": "pkg9 sum",
      "description": "pkg9 desc",
      "homepage": "https://pkg9.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-10-02T15:04:05",
          "epoch": 0,
          "release": "9.fc30",
          "source": {
            "license": "MIT",
            "version": "9.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-10-03T15:04:05",
          "epoch": 0,
          "release": "9.fc30",
          "source": {
            "license": "MIT",
            "version": "9.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    }
  ]
}
`

const projectsInfoFilteredResponse = `
{
  "projects": [
    {
      "name": "package16",
      "summary": "pkg16 sum",
      "description": "pkg16 desc",
      "homepage": "https://pkg16.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-05-02T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-05-03T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package2",
      "summary": "pkg2 sum",
      "description": "pkg2 desc",
      "homepage": "https://pkg2.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-03-02T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-03-03T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package20",
      "summary": "pkg20 sum",
      "description": "pkg20 desc",
      "homepage": "https://pkg20.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-09-02T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-09-03T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package21",
      "summary": "pkg21 sum",
      "description": "pkg21 desc",
      "homepage": "https://pkg21.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-10-02T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-10-03T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    }
  ]
}
`

const projectsInfoPackage16Response = `
{
  "projects": [
    {
      "name": "package16",
      "summary": "pkg16 sum",
      "description": "pkg16 desc",
      "homepage": "https://pkg16.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-05-02T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {},
          "source": {
            "license": "MIT",
            "version": "16.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          }
        },
        {
          "arch": "x86_64",
          "build_time": "2007-05-03T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {},
          "source": {
            "license": "MIT",
            "version": "16.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          }
        }
      ]
    }
  ]
}
`

var modulesInfoResponse = fmt.Sprintf(`
{
  "modules": [
    {
      "name": "package0",
      "summary": "pkg0 sum",
      "description": "pkg0 desc",
      "homepage": "https://pkg0.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-01-02T15:04:05",
          "epoch": 0,
          "release": "0.fc30",
          "source": {
            "license": "MIT",
            "version": "0.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-01-03T15:04:05",
          "epoch": 0,
          "release": "0.fc30",
          "source": {
            "license": "MIT",
            "version": "0.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package1",
      "summary": "pkg1 sum",
      "description": "pkg1 desc",
      "homepage": "https://pkg1.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-02-02T15:04:05",
          "epoch": 0,
          "release": "1.fc30",
          "source": {
            "license": "MIT",
            "version": "1.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-02-03T15:04:05",
          "epoch": 0,
          "release": "1.fc30",
          "source": {
            "license": "MIT",
            "version": "1.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package10",
      "summary": "pkg10 sum",
      "description": "pkg10 desc",
      "homepage": "https://pkg10.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-11-02T15:04:05",
          "epoch": 0,
          "release": "10.fc30",
          "source": {
            "license": "MIT",
            "version": "10.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-11-03T15:04:05",
          "epoch": 0,
          "release": "10.fc30",
          "source": {
            "license": "MIT",
            "version": "10.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package11",
      "summary": "pkg11 sum",
      "description": "pkg11 desc",
      "homepage": "https://pkg11.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-12-02T15:04:05",
          "epoch": 0,
          "release": "11.fc30",
          "source": {
            "license": "MIT",
            "version": "11.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-12-03T15:04:05",
          "epoch": 0,
          "release": "11.fc30",
          "source": {
            "license": "MIT",
            "version": "11.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package12",
      "summary": "pkg12 sum",
      "description": "pkg12 desc",
      "homepage": "https://pkg12.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-01-02T15:04:05",
          "epoch": 0,
          "release": "12.fc30",
          "source": {
            "license": "MIT",
            "version": "12.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-01-03T15:04:05",
          "epoch": 0,
          "release": "12.fc30",
          "source": {
            "license": "MIT",
            "version": "12.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package13",
      "summary": "pkg13 sum",
      "description": "pkg13 desc",
      "homepage": "https://pkg13.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-02-02T15:04:05",
          "epoch": 0,
          "release": "13.fc30",
          "source": {
            "license": "MIT",
            "version": "13.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-02-03T15:04:05",
          "epoch": 0,
          "release": "13.fc30",
          "source": {
            "license": "MIT",
            "version": "13.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package14",
      "summary": "pkg14 sum",
      "description": "pkg14 desc",
      "homepage": "https://pkg14.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-03-02T15:04:05",
          "epoch": 0,
          "release": "14.fc30",
          "source": {
            "license": "MIT",
            "version": "14.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-03-03T15:04:05",
          "epoch": 0,
          "release": "14.fc30",
          "source": {
            "license": "MIT",
            "version": "14.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package15",
      "summary": "pkg15 sum",
      "description": "pkg15 desc",
      "homepage": "https://pkg15.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-04-02T15:04:05",
          "epoch": 0,
          "release": "15.fc30",
          "source": {
            "license": "MIT",
            "version": "15.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-04-03T15:04:05",
          "epoch": 0,
          "release": "15.fc30",
          "source": {
            "license": "MIT",
            "version": "15.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package16",
      "summary": "pkg16 sum",
      "description": "pkg16 desc",
      "homepage": "https://pkg16.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-05-02T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-05-03T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package17",
      "summary": "pkg17 sum",
      "description": "pkg17 desc",
      "homepage": "https://pkg17.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-06-02T15:04:05",
          "epoch": 0,
          "release": "17.fc30",
          "source": {
            "license": "MIT",
            "version": "17.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-06-03T15:04:05",
          "epoch": 0,
          "release": "17.fc30",
          "source": {
            "license": "MIT",
            "version": "17.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package18",
      "summary": "pkg18 sum",
      "description": "pkg18 desc",
      "homepage": "https://pkg18.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-07-02T15:04:05",
          "epoch": 0,
          "release": "18.fc30",
          "source": {
            "license": "MIT",
            "version": "18.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-07-03T15:04:05",
          "epoch": 0,
          "release": "18.fc30",
          "source": {
            "license": "MIT",
            "version": "18.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package19",
      "summary": "pkg19 sum",
      "description": "pkg19 desc",
      "homepage": "https://pkg19.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-08-02T15:04:05",
          "epoch": 0,
          "release": "19.fc30",
          "source": {
            "license": "MIT",
            "version": "19.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-08-03T15:04:05",
          "epoch": 0,
          "release": "19.fc30",
          "source": {
            "license": "MIT",
            "version": "19.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package2",
      "summary": "pkg2 sum",
      "description": "pkg2 desc",
      "homepage": "https://pkg2.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-03-02T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-03-03T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package20",
      "summary": "pkg20 sum",
      "description": "pkg20 desc",
      "homepage": "https://pkg20.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-09-02T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-09-03T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package21",
      "summary": "pkg21 sum",
      "description": "pkg21 desc",
      "homepage": "https://pkg21.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-10-02T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-10-03T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package3",
      "summary": "pkg3 sum",
      "description": "pkg3 desc",
      "homepage": "https://pkg3.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-04-02T15:04:05",
          "epoch": 0,
          "release": "3.fc30",
          "source": {
            "license": "MIT",
            "version": "3.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-04-03T15:04:05",
          "epoch": 0,
          "release": "3.fc30",
          "source": {
            "license": "MIT",
            "version": "3.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package4",
      "summary": "pkg4 sum",
      "description": "pkg4 desc",
      "homepage": "https://pkg4.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-05-02T15:04:05",
          "epoch": 0,
          "release": "4.fc30",
          "source": {
            "license": "MIT",
            "version": "4.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-05-03T15:04:05",
          "epoch": 0,
          "release": "4.fc30",
          "source": {
            "license": "MIT",
            "version": "4.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package5",
      "summary": "pkg5 sum",
      "description": "pkg5 desc",
      "homepage": "https://pkg5.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-06-02T15:04:05",
          "epoch": 0,
          "release": "5.fc30",
          "source": {
            "license": "MIT",
            "version": "5.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-06-03T15:04:05",
          "epoch": 0,
          "release": "5.fc30",
          "source": {
            "license": "MIT",
            "version": "5.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package6",
      "summary": "pkg6 sum",
      "description": "pkg6 desc",
      "homepage": "https://pkg6.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-07-02T15:04:05",
          "epoch": 0,
          "release": "6.fc30",
          "source": {
            "license": "MIT",
            "version": "6.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-07-03T15:04:05",
          "epoch": 0,
          "release": "6.fc30",
          "source": {
            "license": "MIT",
            "version": "6.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package7",
      "summary": "pkg7 sum",
      "description": "pkg7 desc",
      "homepage": "https://pkg7.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-08-02T15:04:05",
          "epoch": 0,
          "release": "7.fc30",
          "source": {
            "license": "MIT",
            "version": "7.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-08-03T15:04:05",
          "epoch": 0,
          "release": "7.fc30",
          "source": {
            "license": "MIT",
            "version": "7.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package8",
      "summary": "pkg8 sum",
      "description": "pkg8 desc",
      "homepage": "https://pkg8.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-09-02T15:04:05",
          "epoch": 0,
          "release": "8.fc30",
          "source": {
            "license": "MIT",
            "version": "8.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-09-03T15:04:05",
          "epoch": 0,
          "release": "8.fc30",
          "source": {
            "license": "MIT",
            "version": "8.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package9",
      "summary": "pkg9 sum",
      "description": "pkg9 desc",
      "homepage": "https://pkg9.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-10-02T15:04:05",
          "epoch": 0,
          "release": "9.fc30",
          "source": {
            "license": "MIT",
            "version": "9.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-10-03T15:04:05",
          "epoch": 0,
          "release": "9.fc30",
          "source": {
            "license": "MIT",
            "version": "9.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    }
  ]
}
`, depsolveDependenciesPartialResponse(testRepoID))

var modulesInfoFilteredResponse = fmt.Sprintf(`
{
  "modules": [
    {
      "name": "package16",
      "summary": "pkg16 sum",
      "description": "pkg16 desc",
      "homepage": "https://pkg16.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-05-02T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-05-03T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package2",
      "summary": "pkg2 sum",
      "description": "pkg2 desc",
      "homepage": "https://pkg2.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-03-02T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-03-03T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package20",
      "summary": "pkg20 sum",
      "description": "pkg20 desc",
      "homepage": "https://pkg20.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-09-02T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-09-03T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    },
    {
      "name": "package21",
      "summary": "pkg21 sum",
      "description": "pkg21 desc",
      "homepage": "https://pkg21.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-10-02T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-10-03T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %[1]s
    }
  ]
}
`, depsolveDependenciesPartialResponse(testRepoID))

var modulesInfoPackage16Response = fmt.Sprintf(`
{
  "modules": [
    {
      "name": "package16",
      "summary": "pkg16 sum",
      "description": "pkg16 desc",
      "homepage": "https://pkg16.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-05-02T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-05-03T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ],
      "dependencies": %s
    }
  ]
}
`, depsolveDependenciesPartialResponse(testRepoID2))

const projectsListResponse = `
{
  "total": 22,
  "offset": 0,
  "limit": 20,
  "projects": [
    {
      "name": "package0",
      "summary": "pkg0 sum",
      "description": "pkg0 desc",
      "homepage": "https://pkg0.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-01-02T15:04:05",
          "epoch": 0,
          "release": "0.fc30",
          "source": {
            "license": "MIT",
            "version": "0.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-01-03T15:04:05",
          "epoch": 0,
          "release": "0.fc30",
          "source": {
            "license": "MIT",
            "version": "0.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package1",
      "summary": "pkg1 sum",
      "description": "pkg1 desc",
      "homepage": "https://pkg1.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-02-02T15:04:05",
          "epoch": 0,
          "release": "1.fc30",
          "source": {
            "license": "MIT",
            "version": "1.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-02-03T15:04:05",
          "epoch": 0,
          "release": "1.fc30",
          "source": {
            "license": "MIT",
            "version": "1.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package10",
      "summary": "pkg10 sum",
      "description": "pkg10 desc",
      "homepage": "https://pkg10.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-11-02T15:04:05",
          "epoch": 0,
          "release": "10.fc30",
          "source": {
            "license": "MIT",
            "version": "10.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-11-03T15:04:05",
          "epoch": 0,
          "release": "10.fc30",
          "source": {
            "license": "MIT",
            "version": "10.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package11",
      "summary": "pkg11 sum",
      "description": "pkg11 desc",
      "homepage": "https://pkg11.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-12-02T15:04:05",
          "epoch": 0,
          "release": "11.fc30",
          "source": {
            "license": "MIT",
            "version": "11.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-12-03T15:04:05",
          "epoch": 0,
          "release": "11.fc30",
          "source": {
            "license": "MIT",
            "version": "11.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package12",
      "summary": "pkg12 sum",
      "description": "pkg12 desc",
      "homepage": "https://pkg12.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-01-02T15:04:05",
          "epoch": 0,
          "release": "12.fc30",
          "source": {
            "license": "MIT",
            "version": "12.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-01-03T15:04:05",
          "epoch": 0,
          "release": "12.fc30",
          "source": {
            "license": "MIT",
            "version": "12.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package13",
      "summary": "pkg13 sum",
      "description": "pkg13 desc",
      "homepage": "https://pkg13.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-02-02T15:04:05",
          "epoch": 0,
          "release": "13.fc30",
          "source": {
            "license": "MIT",
            "version": "13.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-02-03T15:04:05",
          "epoch": 0,
          "release": "13.fc30",
          "source": {
            "license": "MIT",
            "version": "13.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package14",
      "summary": "pkg14 sum",
      "description": "pkg14 desc",
      "homepage": "https://pkg14.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-03-02T15:04:05",
          "epoch": 0,
          "release": "14.fc30",
          "source": {
            "license": "MIT",
            "version": "14.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-03-03T15:04:05",
          "epoch": 0,
          "release": "14.fc30",
          "source": {
            "license": "MIT",
            "version": "14.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package15",
      "summary": "pkg15 sum",
      "description": "pkg15 desc",
      "homepage": "https://pkg15.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-04-02T15:04:05",
          "epoch": 0,
          "release": "15.fc30",
          "source": {
            "license": "MIT",
            "version": "15.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-04-03T15:04:05",
          "epoch": 0,
          "release": "15.fc30",
          "source": {
            "license": "MIT",
            "version": "15.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package16",
      "summary": "pkg16 sum",
      "description": "pkg16 desc",
      "homepage": "https://pkg16.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-05-02T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-05-03T15:04:05",
          "epoch": 0,
          "release": "16.fc30",
          "source": {
            "license": "MIT",
            "version": "16.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package17",
      "summary": "pkg17 sum",
      "description": "pkg17 desc",
      "homepage": "https://pkg17.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-06-02T15:04:05",
          "epoch": 0,
          "release": "17.fc30",
          "source": {
            "license": "MIT",
            "version": "17.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-06-03T15:04:05",
          "epoch": 0,
          "release": "17.fc30",
          "source": {
            "license": "MIT",
            "version": "17.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package18",
      "summary": "pkg18 sum",
      "description": "pkg18 desc",
      "homepage": "https://pkg18.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-07-02T15:04:05",
          "epoch": 0,
          "release": "18.fc30",
          "source": {
            "license": "MIT",
            "version": "18.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-07-03T15:04:05",
          "epoch": 0,
          "release": "18.fc30",
          "source": {
            "license": "MIT",
            "version": "18.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package19",
      "summary": "pkg19 sum",
      "description": "pkg19 desc",
      "homepage": "https://pkg19.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-08-02T15:04:05",
          "epoch": 0,
          "release": "19.fc30",
          "source": {
            "license": "MIT",
            "version": "19.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-08-03T15:04:05",
          "epoch": 0,
          "release": "19.fc30",
          "source": {
            "license": "MIT",
            "version": "19.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package2",
      "summary": "pkg2 sum",
      "description": "pkg2 desc",
      "homepage": "https://pkg2.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-03-02T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-03-03T15:04:05",
          "epoch": 0,
          "release": "2.fc30",
          "source": {
            "license": "MIT",
            "version": "2.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package20",
      "summary": "pkg20 sum",
      "description": "pkg20 desc",
      "homepage": "https://pkg20.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-09-02T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-09-03T15:04:05",
          "epoch": 0,
          "release": "20.fc30",
          "source": {
            "license": "MIT",
            "version": "20.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package21",
      "summary": "pkg21 sum",
      "description": "pkg21 desc",
      "homepage": "https://pkg21.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2007-10-02T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2007-10-03T15:04:05",
          "epoch": 0,
          "release": "21.fc30",
          "source": {
            "license": "MIT",
            "version": "21.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package3",
      "summary": "pkg3 sum",
      "description": "pkg3 desc",
      "homepage": "https://pkg3.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-04-02T15:04:05",
          "epoch": 0,
          "release": "3.fc30",
          "source": {
            "license": "MIT",
            "version": "3.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-04-03T15:04:05",
          "epoch": 0,
          "release": "3.fc30",
          "source": {
            "license": "MIT",
            "version": "3.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package4",
      "summary": "pkg4 sum",
      "description": "pkg4 desc",
      "homepage": "https://pkg4.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-05-02T15:04:05",
          "epoch": 0,
          "release": "4.fc30",
          "source": {
            "license": "MIT",
            "version": "4.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-05-03T15:04:05",
          "epoch": 0,
          "release": "4.fc30",
          "source": {
            "license": "MIT",
            "version": "4.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package5",
      "summary": "pkg5 sum",
      "description": "pkg5 desc",
      "homepage": "https://pkg5.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-06-02T15:04:05",
          "epoch": 0,
          "release": "5.fc30",
          "source": {
            "license": "MIT",
            "version": "5.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-06-03T15:04:05",
          "epoch": 0,
          "release": "5.fc30",
          "source": {
            "license": "MIT",
            "version": "5.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package6",
      "summary": "pkg6 sum",
      "description": "pkg6 desc",
      "homepage": "https://pkg6.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-07-02T15:04:05",
          "epoch": 0,
          "release": "6.fc30",
          "source": {
            "license": "MIT",
            "version": "6.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-07-03T15:04:05",
          "epoch": 0,
          "release": "6.fc30",
          "source": {
            "license": "MIT",
            "version": "6.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    },
    {
      "name": "package7",
      "summary": "pkg7 sum",
      "description": "pkg7 desc",
      "homepage": "https://pkg7.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-08-02T15:04:05",
          "epoch": 0,
          "release": "7.fc30",
          "source": {
            "license": "MIT",
            "version": "7.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-08-03T15:04:05",
          "epoch": 0,
          "release": "7.fc30",
          "source": {
            "license": "MIT",
            "version": "7.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    }
  ]
}
`

const projectsList1Response = `
{
  "total": 22,
  "offset": 1,
  "limit": 1,
  "projects": [
    {
      "name": "package1",
      "summary": "pkg1 sum",
      "description": "pkg1 desc",
      "homepage": "https://pkg1.example.com",
      "upstream_vcs": "UPSTREAM_VCS",
      "builds": [
        {
          "arch": "x86_64",
          "build_time": "2006-02-02T15:04:05",
          "epoch": 0,
          "release": "1.fc30",
          "source": {
            "license": "MIT",
            "version": "1.0",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        },
        {
          "arch": "x86_64",
          "build_time": "2006-02-03T15:04:05",
          "epoch": 0,
          "release": "1.fc30",
          "source": {
            "license": "MIT",
            "version": "1.1",
            "source_ref": "SOURCE_REF",
            "metadata": {}
          },
          "changelog": "CHANGELOG_NEEDED",
          "build_config_ref": "BUILD_CONFIG_REF",
          "build_env_ref": "BUILD_ENV_REF",
          "metadata": {}
        }
      ]
    }
  ]
}
`

const modulesListResponse = `
{
  "total": 22,
  "offset": 0,
  "limit": 20,
  "modules": [
    {
      "name": "package0",
      "group_type": "rpm"
    },
    {
      "name": "package1",
      "group_type": "rpm"
    },
    {
      "name": "package10",
      "group_type": "rpm"
    },
    {
      "name": "package11",
      "group_type": "rpm"
    },
    {
      "name": "package12",
      "group_type": "rpm"
    },
    {
      "name": "package13",
      "group_type": "rpm"
    },
    {
      "name": "package14",
      "group_type": "rpm"
    },
    {
      "name": "package15",
      "group_type": "rpm"
    },
    {
      "name": "package16",
      "group_type": "rpm"
    },
    {
      "name": "package17",
      "group_type": "rpm"
    },
    {
      "name": "package18",
      "group_type": "rpm"
    },
    {
      "name": "package19",
      "group_type": "rpm"
    },
    {
      "name": "package2",
      "group_type": "rpm"
    },
    {
      "name": "package20",
      "group_type": "rpm"
    },
    {
      "name": "package21",
      "group_type": "rpm"
    },
    {
      "name": "package3",
      "group_type": "rpm"
    },
    {
      "name": "package4",
      "group_type": "rpm"
    },
    {
      "name": "package5",
      "group_type": "rpm"
    },
    {
      "name": "package6",
      "group_type": "rpm"
    },
    {
      "name": "package7",
      "group_type": "rpm"
    }
  ]
}
`

const modulesListFilteredResponse = `
{
  "total": 4,
  "offset": 0,
  "limit": 20,
  "modules": [
    {
      "name": "package16",
      "group_type": "rpm"
    },
    {
      "name": "package2",
      "group_type": "rpm"
    },
    {
      "name": "package20",
      "group_type": "rpm"
    },
    {
      "name": "package21",
      "group_type": "rpm"
    }
  ]
}
`
