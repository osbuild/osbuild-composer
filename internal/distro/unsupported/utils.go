package unsupported

import (
	"io/ioutil"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func GenerateFedoraRepositories(version string) map[string][]rpmmd.RepoConfig {
	content, err := ioutil.ReadFile("/etc/pki/rpm-gpg/RPM-GPG-KEY-fedora-" + version + "-primary")
	common.PanicOnError(err)

	key_text := string(content)
	all_repos := make(map[string][]rpmmd.RepoConfig)
	for _, arch := range []string{"x86_64", "aarch64"} {
		repos := []rpmmd.RepoConfig{}
		for _, repo_name := range []string{"fedora-", "updates-released-f", "fedora-modular-", "updates-released-modular-f"} {
			repos = append(repos, rpmmd.RepoConfig{
				Name:     repo_name + version,
				Metalink: "https://mirrors.fedoraproject.org/metalink?repo=" + repo_name + version + "&arch=" + arch,
				GPGKey:   key_text,
				CheckGPG: true,
			})
		}
		all_repos[arch] = repos
	}

	return all_repos
}
