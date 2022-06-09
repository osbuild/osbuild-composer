package dnfjson

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/mocks/rpmrepo"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/stretchr/testify/assert"
)

var forceDNF = flag.Bool("force-dnf", false, "force dnf testing, making them fail instead of skip if dnf isn't installed")

func dnfInstalled() bool {
	cmd := exec.Command("python3", "-c", "import dnf")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to import dnf: %s\n", err.Error())
		return false
	}
	return true
}

func TestDepsolver(t *testing.T) {
	if !*forceDNF {
		// dnf tests aren't forced: skip them if the dnf sniff check fails
		if !dnfInstalled() {
			t.Skip()
		}
	}

	s := rpmrepo.NewTestServer()
	defer s.Close()

	assert := assert.New(t)

	tmpdir := t.TempDir()
	solver := NewSolver("platform:el9", "9", "x86_64", tmpdir)
	solver.SetDNFJSONPath("../../dnf-json")

	{ // single depsolve
		pkgsets := []rpmmd.PackageSet{{Include: []string{"kernel", "vim-minimal", "tmux", "zsh"}, Repositories: []rpmmd.RepoConfig{s.RepoConfig}}} // everything you'll ever need

		deps, err := solver.Depsolve(pkgsets)
		if err != nil {
			t.Fatal(err)
		}
		exp := expectedResult(s.RepoConfig)
		assert.Equal(deps, exp)
	}

	{ // chain depsolve of the same packages in order should produce the same result (at least in this case)
		pkgsets := []rpmmd.PackageSet{
			{Include: []string{"kernel"}, Repositories: []rpmmd.RepoConfig{s.RepoConfig}},
			{Include: []string{"vim-minimal", "tmux", "zsh"}, Repositories: []rpmmd.RepoConfig{s.RepoConfig}},
		}
		deps, err := solver.Depsolve(pkgsets)
		if err != nil {
			t.Fatal(err)
		}
		exp := expectedResult(s.RepoConfig)
		assert.Equal(deps, exp)
	}
}

func TestMakeDepsolveRequest(t *testing.T) {

	baseOS := rpmmd.RepoConfig{
		Name:    "baseos",
		BaseURL: "https://example.org/baseos",
	}
	appstream := rpmmd.RepoConfig{
		Name:    "appstream",
		BaseURL: "https://example.org/appstream",
	}
	userRepo := rpmmd.RepoConfig{
		Name:    "user-repo",
		BaseURL: "https://example.org/user-repo",
	}
	userRepo2 := rpmmd.RepoConfig{
		Name:    "user-repo-2",
		BaseURL: "https://example.org/user-repo-2",
	}
	tests := []struct {
		packageSets []rpmmd.PackageSet
		args        []transactionArgs
		wantRepos   []repoConfig
		err         bool
	}{
		// single transaction
		{
			packageSets: []rpmmd.PackageSet{
				{
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
					Repositories: []rpmmd.RepoConfig{
						baseOS,
						appstream,
					},
				},
			},
			args: []transactionArgs{
				{
					PackageSpecs: []string{"pkg1"},
					ExcludeSpecs: []string{"pkg2"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash()},
				},
			},
			wantRepos: []repoConfig{
				{
					ID:      baseOS.Hash(),
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					ID:      appstream.Hash(),
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
		},
		// 2 transactions + package set specific repo
		{
			packageSets: []rpmmd.PackageSet{
				{
					Include:      []string{"pkg1"},
					Exclude:      []string{"pkg2"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
			},
			args: []transactionArgs{
				{
					PackageSpecs: []string{"pkg1"},
					ExcludeSpecs: []string{"pkg2"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash()},
				},
				{
					PackageSpecs: []string{"pkg3"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash(), userRepo.Hash()},
				},
			},
			wantRepos: []repoConfig{
				{
					ID:      baseOS.Hash(),
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					ID:      appstream.Hash(),
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
				{
					ID:      userRepo.Hash(),
					Name:    "user-repo",
					BaseURL: "https://example.org/user-repo",
				},
			},
		},
		// 2 transactions + no package set specific repos
		{
			packageSets: []rpmmd.PackageSet{
				{
					Include:      []string{"pkg1"},
					Exclude:      []string{"pkg2"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
			},
			args: []transactionArgs{
				{
					PackageSpecs: []string{"pkg1"},
					ExcludeSpecs: []string{"pkg2"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash()},
				},
				{
					PackageSpecs: []string{"pkg3"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash()},
				},
			},
			wantRepos: []repoConfig{
				{
					ID:      baseOS.Hash(),
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					ID:      appstream.Hash(),
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
		},
		// 3 transactions + package set specific repo used by 2nd and 3rd transaction
		{
			packageSets: []rpmmd.PackageSet{
				{
					Include:      []string{"pkg1"},
					Exclude:      []string{"pkg2"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
				{
					Include:      []string{"pkg4"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
			},
			args: []transactionArgs{
				{
					PackageSpecs: []string{"pkg1"},
					ExcludeSpecs: []string{"pkg2"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash()},
				},
				{
					PackageSpecs: []string{"pkg3"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash(), userRepo.Hash()},
				},
				{
					PackageSpecs: []string{"pkg4"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash(), userRepo.Hash()},
				},
			},
			wantRepos: []repoConfig{
				{
					ID:      baseOS.Hash(),
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					ID:      appstream.Hash(),
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
				{
					ID:      userRepo.Hash(),
					Name:    "user-repo",
					BaseURL: "https://example.org/user-repo",
				},
			},
		},
		// 3 transactions + package set specific repo used by 2nd and 3rd transaction
		// + 3rd transaction using another repo
		{
			packageSets: []rpmmd.PackageSet{
				{
					Include:      []string{"pkg1"},
					Exclude:      []string{"pkg2"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
				{
					Include:      []string{"pkg4"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo, userRepo2},
				},
			},
			args: []transactionArgs{
				{
					PackageSpecs: []string{"pkg1"},
					ExcludeSpecs: []string{"pkg2"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash()},
				},
				{
					PackageSpecs: []string{"pkg3"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash(), userRepo.Hash()},
				},
				{
					PackageSpecs: []string{"pkg4"},
					RepoIDs:      []string{baseOS.Hash(), appstream.Hash(), userRepo.Hash(), userRepo2.Hash()},
				},
			},
			wantRepos: []repoConfig{
				{
					ID:      baseOS.Hash(),
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					ID:      appstream.Hash(),
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
				{
					ID:      userRepo.Hash(),
					Name:    "user-repo",
					BaseURL: "https://example.org/user-repo",
				},
				{
					ID:      userRepo2.Hash(),
					Name:    "user-repo-2",
					BaseURL: "https://example.org/user-repo-2",
				},
			},
		},
		// Error: 3 transactions + 3rd one not using repo used by 2nd one
		{
			packageSets: []rpmmd.PackageSet{
				{
					Include:      []string{"pkg1"},
					Exclude:      []string{"pkg2"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo},
				},
				{
					Include:      []string{"pkg4"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo2},
				},
			},
			err: true,
		},
		// Error: 3 transactions but last one doesn't specify user repos in 2nd
		{
			packageSets: []rpmmd.PackageSet{
				{
					Include:      []string{"pkg1"},
					Exclude:      []string{"pkg2"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
				{
					Include:      []string{"pkg3"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream, userRepo, userRepo2},
				},
				{
					Include:      []string{"pkg4"},
					Repositories: []rpmmd.RepoConfig{baseOS, appstream},
				},
			},
			err: true,
		},
	}
	solver := NewSolver("", "", "", "")
	for idx, tt := range tests {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			req, _, err := solver.makeDepsolveRequest(tt.packageSets)
			if tt.err {
				assert.NotNilf(t, err, "expected an error, but got 'nil' instead")
				assert.Nilf(t, req, "got non-nill request, but expected an error")
			} else {
				assert.Nilf(t, err, "expected 'nil', but got error instead")
				assert.NotNilf(t, req, "expected non-nill request, but got 'nil' instead")

				assert.Equal(t, tt.args, req.Arguments.Transactions)
				assert.Equal(t, tt.wantRepos, req.Arguments.Repos)
			}
		})
	}
}

func expectedResult(repo rpmmd.RepoConfig) []rpmmd.PackageSpec {
	// need to change the url for the RemoteLocation and the repo ID since the port is different each time and we don't want to have a fixed one
	expectedTemplate := []rpmmd.PackageSpec{
		{Name: "acl", Epoch: 0, Version: "2.3.1", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/acl-2.3.1-3.el9.x86_64.rpm", Checksum: "sha256:986044c3837eddbc9231d7be5e5fc517e245296978b988a803bc9f9172fe84ea", Secrets: "", CheckGPG: false},
		{Name: "alternatives", Epoch: 0, Version: "1.20", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/alternatives-1.20-2.el9.x86_64.rpm", Checksum: "sha256:1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db", Secrets: "", CheckGPG: false},
		{Name: "audit-libs", Epoch: 0, Version: "3.0.7", Release: "100.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/audit-libs-3.0.7-100.el9.x86_64.rpm", Checksum: "sha256:a4bdda48abaedffeb74398cd55afbd00cb4153ae24bd2a3e6de9d87462df5ffa", Secrets: "", CheckGPG: false},
		{Name: "basesystem", Epoch: 0, Version: "11", Release: "13.el9", Arch: "noarch", RemoteLocation: "%s/Packages/basesystem-11-13.el9.noarch.rpm", Checksum: "sha256:a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973", Secrets: "", CheckGPG: false},
		{Name: "bash", Epoch: 0, Version: "5.1.8", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/bash-5.1.8-2.el9.x86_64.rpm", Checksum: "sha256:3d45552ea940db0556dd2dc73e92c20c0d7cbc9e617f251904f20475d4ecc6b6", Secrets: "", CheckGPG: false},
		{Name: "bzip2-libs", Epoch: 0, Version: "1.0.8", Release: "8.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/bzip2-libs-1.0.8-8.el9.x86_64.rpm", Checksum: "sha256:fabd6b5c065c2b9d4a8d39a938ae577d801de2ddc73c8cdf6f7803db29c28d0a", Secrets: "", CheckGPG: false},
		{Name: "ca-certificates", Epoch: 0, Version: "2020.2.50", Release: "94.el9", Arch: "noarch", RemoteLocation: "%s/Packages/ca-certificates-2020.2.50-94.el9.noarch.rpm", Checksum: "sha256:3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09", Secrets: "", CheckGPG: false},
		{Name: "centos-gpg-keys", Epoch: 0, Version: "9.0", Release: "9.el9", Arch: "noarch", RemoteLocation: "%s/Packages/centos-gpg-keys-9.0-9.el9.noarch.rpm", Checksum: "sha256:2785ab660c124c9bda4ef4057e72d7fc73e8ac254ddd09a5541a6d323740dad7", Secrets: "", CheckGPG: false},
		{Name: "centos-stream-release", Epoch: 0, Version: "9.0", Release: "9.el9", Arch: "noarch", RemoteLocation: "%s/Packages/centos-stream-release-9.0-9.el9.noarch.rpm", Checksum: "sha256:44246cc9b62ac0fb833ece49cff6ac0a932234fcba26b8c895f42baebf0a19c2", Secrets: "", CheckGPG: false},
		{Name: "centos-stream-repos", Epoch: 0, Version: "9.0", Release: "9.el9", Arch: "noarch", RemoteLocation: "%s/Packages/centos-stream-repos-9.0-9.el9.noarch.rpm", Checksum: "sha256:90208bb7dd1558a3311a28ea06d75ad7e83be3f223c5fb2eff1b9ac47bb98ebe", Secrets: "", CheckGPG: false},
		{Name: "coreutils", Epoch: 0, Version: "8.32", Release: "31.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/coreutils-8.32-31.el9.x86_64.rpm", Checksum: "sha256:647a3b9a52df25cb2aaf7f3715b219839b4cf71913638c88172d925173280812", Secrets: "", CheckGPG: false},
		{Name: "coreutils-common", Epoch: 0, Version: "8.32", Release: "31.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/coreutils-common-8.32-31.el9.x86_64.rpm", Checksum: "sha256:864b166ac6d55cad5010da369fd7ad4872f81c2c111867dfbf96ccf4c8273c7e", Secrets: "", CheckGPG: false},
		{Name: "cpio", Epoch: 0, Version: "2.13", Release: "16.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/cpio-2.13-16.el9.x86_64.rpm", Checksum: "sha256:216b76d33443b732be42fe1d443e106a17e632ac9ca465928c37a8c0ede596a4", Secrets: "", CheckGPG: false},
		{Name: "cracklib", Epoch: 0, Version: "2.9.6", Release: "27.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/cracklib-2.9.6-27.el9.x86_64.rpm", Checksum: "sha256:be9deb2efd06b4b2c1c130acae94c687161d04830119e65a989d904ba9fd1864", Secrets: "", CheckGPG: false},
		{Name: "cracklib-dicts", Epoch: 0, Version: "2.9.6", Release: "27.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/cracklib-dicts-2.9.6-27.el9.x86_64.rpm", Checksum: "sha256:01df2a72fcdf988132e82764ce1a22a5a9513fa253b54e17d23058bdb53c2d85", Secrets: "", CheckGPG: false},
		{Name: "crypto-policies", Epoch: 0, Version: "20220203", Release: "1.gitf03e75e.el9", Arch: "noarch", RemoteLocation: "%s/Packages/crypto-policies-20220203-1.gitf03e75e.el9.noarch.rpm", Checksum: "sha256:28d73d3800cb895b265bc0755c41241c593ebd7551a7da7f001f1e254b85c662", Secrets: "", CheckGPG: false},
		{Name: "cryptsetup-libs", Epoch: 0, Version: "2.4.3", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/cryptsetup-libs-2.4.3-1.el9.x86_64.rpm", Checksum: "sha256:6919d88afdd2cf89982ec8edd4401ff93394a81873f81cf89bb273384f39104f", Secrets: "", CheckGPG: false},
		{Name: "dbus", Epoch: 1, Version: "1.12.20", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/dbus-1.12.20-5.el9.x86_64.rpm", Checksum: "sha256:bb85bd28cc162e98da53b756b988ffd9350f4dbcc186f4c6962ae047e27f83d3", Secrets: "", CheckGPG: false},
		{Name: "dbus-broker", Epoch: 0, Version: "28", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/dbus-broker-28-5.el9.x86_64.rpm", Checksum: "sha256:e9efdcdcfe430e474e3a7f09596a0a5a4314692d9ae846bb1ca86ff88ef81038", Secrets: "", CheckGPG: false},
		{Name: "dbus-common", Epoch: 1, Version: "1.12.20", Release: "5.el9", Arch: "noarch", RemoteLocation: "%s/Packages/dbus-common-1.12.20-5.el9.noarch.rpm", Checksum: "sha256:150048b6fdafd4271bd6badab3f8a2e56b86967266f890770eab7578289cc773", Secrets: "", CheckGPG: false},
		{Name: "device-mapper", Epoch: 9, Version: "1.02.181", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/device-mapper-1.02.181-3.el9.x86_64.rpm", Checksum: "sha256:5d1cd7733f147020ef3a9e08fa2e9d74a25e0ac89dfbadc69912541286146feb", Secrets: "", CheckGPG: false},
		{Name: "device-mapper-libs", Epoch: 9, Version: "1.02.181", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/device-mapper-libs-1.02.181-3.el9.x86_64.rpm", Checksum: "sha256:a716ccca85fad2885af4d099f8c213eb4617d637d8ca6cf7d2b483b9de88a5d3", Secrets: "", CheckGPG: false},
		{Name: "dracut", Epoch: 0, Version: "055", Release: "10.git20210824.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/dracut-055-10.git20210824.el9.x86_64.rpm", Checksum: "sha256:54015283e7f85fbee9d8a814c3bd60c7f81a6eb2ff480ca689e4526637a81c83", Secrets: "", CheckGPG: false},
		{Name: "elfutils-default-yama-scope", Epoch: 0, Version: "0.186", Release: "1.el9", Arch: "noarch", RemoteLocation: "%s/Packages/elfutils-default-yama-scope-0.186-1.el9.noarch.rpm", Checksum: "sha256:0d2dcfaa16f83de78a251cf0b9a4c5e4ec7d4deb2e8d1cae7209be7745fabeb5", Secrets: "", CheckGPG: false},
		{Name: "elfutils-libelf", Epoch: 0, Version: "0.186", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/elfutils-libelf-0.186-1.el9.x86_64.rpm", Checksum: "sha256:0e295e6150b6929408ac29792ec5f3ebeb4a20607eb553177f0e4899b3008d63", Secrets: "", CheckGPG: false},
		{Name: "elfutils-libs", Epoch: 0, Version: "0.186", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/elfutils-libs-0.186-1.el9.x86_64.rpm", Checksum: "sha256:bcc47b8ab496d3d11d772b037e022bc3a4ce3b080b7d1c24fa7f999426a6b8f3", Secrets: "", CheckGPG: false},
		{Name: "expat", Epoch: 0, Version: "2.2.10", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/expat-2.2.10-5.el9.x86_64.rpm", Checksum: "sha256:f97cd3c1e79b4dfff232ba0208c2e1a7d557608c1c37e8303de4f75387be9bb7", Secrets: "", CheckGPG: false},
		{Name: "filesystem", Epoch: 0, Version: "3.16", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/filesystem-3.16-2.el9.x86_64.rpm", Checksum: "sha256:b69a472751268a1b9acd566dc7aa486fc1d6c8cb6d23f36d6a6dfead62e71475", Secrets: "", CheckGPG: false},
		{Name: "findutils", Epoch: 1, Version: "4.8.0", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/findutils-4.8.0-5.el9.x86_64.rpm", Checksum: "sha256:552548e6d6f9623ccd9d31bb185bba3a66730da6e9d02296b417d501356c3848", Secrets: "", CheckGPG: false},
		{Name: "gdbm-libs", Epoch: 1, Version: "1.19", Release: "4.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/gdbm-libs-1.19-4.el9.x86_64.rpm", Checksum: "sha256:8cd5a78cab8783dd241c52c4fcda28fb111c443887dd6d0fe38385e8383c98b3", Secrets: "", CheckGPG: false},
		{Name: "glibc", Epoch: 0, Version: "2.34", Release: "21.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/glibc-2.34-21.el9.x86_64.rpm", Checksum: "sha256:6e40002c40b2e142dac88fba59d9893054b364585b2bc4b63ebf4cb3066616e2", Secrets: "", CheckGPG: false},
		{Name: "glibc-common", Epoch: 0, Version: "2.34", Release: "21.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/glibc-common-2.34-21.el9.x86_64.rpm", Checksum: "sha256:606fda6e7bbe188920afcae1529967fc13c10763ed727d8ac6ce1037a8549228", Secrets: "", CheckGPG: false},
		{Name: "glibc-gconv-extra", Epoch: 0, Version: "2.34", Release: "21.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/glibc-gconv-extra-2.34-21.el9.x86_64.rpm", Checksum: "sha256:cc159162a083a3adf927bcf36fe4c053f3dd3640ff2f7c544018d354e046eccb", Secrets: "", CheckGPG: false},
		{Name: "glibc-minimal-langpack", Epoch: 0, Version: "2.34", Release: "21.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/glibc-minimal-langpack-2.34-21.el9.x86_64.rpm", Checksum: "sha256:bfec403288415b69acb3fd4bd014561d639673c7002c6968e3722e88cb104bdc", Secrets: "", CheckGPG: false},
		{Name: "gmp", Epoch: 1, Version: "6.2.0", Release: "10.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/gmp-6.2.0-10.el9.x86_64.rpm", Checksum: "sha256:1a6ededc80029ef258288ddbf24bcce7c6228647841416950c88e3f14b7258a2", Secrets: "", CheckGPG: false},
		{Name: "grep", Epoch: 0, Version: "3.6", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/grep-3.6-5.el9.x86_64.rpm", Checksum: "sha256:10a41b66b1fbd6eb055178e22c37199e5b49b4852e77c806f7af7211044a4a55", Secrets: "", CheckGPG: false},
		{Name: "gzip", Epoch: 0, Version: "1.10", Release: "8.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/gzip-1.10-8.el9.x86_64.rpm", Checksum: "sha256:3b5ce98a03a3336a3f32ac7a0867fbc23da702e8618bfd20d49d882d42a460f4", Secrets: "", CheckGPG: false},
		{Name: "json-c", Epoch: 0, Version: "0.14", Release: "11.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/json-c-0.14-11.el9.x86_64.rpm", Checksum: "sha256:1a75404c6bc8c1369914077dc99480e73bf13a40f15fd1cd8afc792b8600adf8", Secrets: "", CheckGPG: false},
		{Name: "kbd", Epoch: 0, Version: "2.4.0", Release: "8.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/kbd-2.4.0-8.el9.x86_64.rpm", Checksum: "sha256:9c7395caebf76e15f496d9dc7690d772cb34f29d3f6626086b578565e412df51", Secrets: "", CheckGPG: false},
		{Name: "kbd-misc", Epoch: 0, Version: "2.4.0", Release: "8.el9", Arch: "noarch", RemoteLocation: "%s/Packages/kbd-misc-2.4.0-8.el9.noarch.rpm", Checksum: "sha256:2dda3fe56c9a5bce5880dca58d905682c5e9f94ee023e43a3e311d2d411e1849", Secrets: "", CheckGPG: false},
		{Name: "kernel", Epoch: 0, Version: "5.14.0", Release: "55.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/kernel-5.14.0-55.el9.x86_64.rpm", Checksum: "sha256:be5dba9121cda9eac9cc8f20b24f7e0d198364b370546c3665e4e6ce70a335b4", Secrets: "", CheckGPG: false},
		{Name: "kernel-core", Epoch: 0, Version: "5.14.0", Release: "55.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/kernel-core-5.14.0-55.el9.x86_64.rpm", Checksum: "sha256:0afe6e35348485ae2696e6170dcf34370f33fcf42a357fc815e332d939dd1025", Secrets: "", CheckGPG: false},
		{Name: "kernel-modules", Epoch: 0, Version: "5.14.0", Release: "55.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/kernel-modules-5.14.0-55.el9.x86_64.rpm", Checksum: "sha256:0914a0cbe0304289e224789f8e50e3e48a2525eba742ad764a1901e8c1351fb5", Secrets: "", CheckGPG: false},
		{Name: "kmod", Epoch: 0, Version: "28", Release: "7.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/kmod-28-7.el9.x86_64.rpm", Checksum: "sha256:3d4bc7935959a109a10020d0d19a5e059719ae4c99c5f32d3020ff6da47d53ea", Secrets: "", CheckGPG: false},
		{Name: "kmod-libs", Epoch: 0, Version: "28", Release: "7.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/kmod-libs-28-7.el9.x86_64.rpm", Checksum: "sha256:0727ff3131223446158aaec88cbf8f894a9e3592e73f231a1802629518eeb64b", Secrets: "", CheckGPG: false},
		{Name: "kpartx", Epoch: 0, Version: "0.8.7", Release: "4.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/kpartx-0.8.7-4.el9.x86_64.rpm", Checksum: "sha256:8f05761c418a55f811404dc1515b131bafe9b1e3fe56274be6d880c8822984b5", Secrets: "", CheckGPG: false},
		{Name: "libacl", Epoch: 0, Version: "2.3.1", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libacl-2.3.1-3.el9.x86_64.rpm", Checksum: "sha256:fd829e9a03f6d321313002d6fcb37ee0434f548aa75fcd3ecdbdd891115de6a7", Secrets: "", CheckGPG: false},
		{Name: "libattr", Epoch: 0, Version: "2.5.1", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libattr-2.5.1-3.el9.x86_64.rpm", Checksum: "sha256:d4db095a015e84065f27a642ee7829cd1690041ba8c51501f908cc34760c9409", Secrets: "", CheckGPG: false},
		{Name: "libblkid", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libblkid-2.37.2-1.el9.x86_64.rpm", Checksum: "sha256:f5cf36e8081c2d72e9dd64dd1614155857dd6e71ebb2237e5b0e11ace5481bac", Secrets: "", CheckGPG: false},
		{Name: "libcap", Epoch: 0, Version: "2.48", Release: "8.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libcap-2.48-8.el9.x86_64.rpm", Checksum: "sha256:c41f91075ee8ca480c2631a485bcc74876b9317b4dc9bd66566da32313621bd7", Secrets: "", CheckGPG: false},
		{Name: "libcap-ng", Epoch: 0, Version: "0.8.2", Release: "6.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libcap-ng-0.8.2-6.el9.x86_64.rpm", Checksum: "sha256:0ee8b2d02fd362223fcf36c11297e1f9ae939f76cef09c0bce9cad5f53287122", Secrets: "", CheckGPG: false},
		{Name: "libdb", Epoch: 0, Version: "5.3.28", Release: "53.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libdb-5.3.28-53.el9.x86_64.rpm", Checksum: "sha256:3a44d15d695944bde4e7290800b815f98bfd9cd6f6f868cec3e8991606f556d5", Secrets: "", CheckGPG: false},
		{Name: "libeconf", Epoch: 0, Version: "0.4.1", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libeconf-0.4.1-2.el9.x86_64.rpm", Checksum: "sha256:1d6fe169e74daff38ad5b0d6424c4d1b14545d5974c39e4421d20838a68f5892", Secrets: "", CheckGPG: false},
		{Name: "libevent", Epoch: 0, Version: "2.1.12", Release: "6.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libevent-2.1.12-6.el9.x86_64.rpm", Checksum: "sha256:82179f6f214ddf523e143c16c3474ccf8832551c6305faf89edfbd83b3424d48", Secrets: "", CheckGPG: false},
		{Name: "libfdisk", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libfdisk-2.37.2-1.el9.x86_64.rpm", Checksum: "sha256:a41bad6e261c719224abfd6745ccb1d2a0cac9d024ca9656904001a38d7cd8c7", Secrets: "", CheckGPG: false},
		{Name: "libffi", Epoch: 0, Version: "3.4.2", Release: "7.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libffi-3.4.2-7.el9.x86_64.rpm", Checksum: "sha256:f0ac4b6454d4018833dd10e3f437d8271c7c6a628d99b37e75b83af890b86bc4", Secrets: "", CheckGPG: false},
		{Name: "libgcc", Epoch: 0, Version: "11.2.1", Release: "9.1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libgcc-11.2.1-9.1.el9.x86_64.rpm", Checksum: "sha256:6fc0ea086ecf7ae65bdfc2e9ba6503ee9d9bf717f3c0a55c4bc9c99e12608edf", Secrets: "", CheckGPG: false},
		{Name: "libgcrypt", Epoch: 0, Version: "1.10.0", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libgcrypt-1.10.0-1.el9.x86_64.rpm", Checksum: "sha256:059533802d440244c1fb6f777e20ed445220cdc85300e164d8ffb0ecfdfb42f4", Secrets: "", CheckGPG: false},
		{Name: "libgpg-error", Epoch: 0, Version: "1.42", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libgpg-error-1.42-5.el9.x86_64.rpm", Checksum: "sha256:a1883804c376f737109f4dff06077d1912b90150a732d11be7bc5b3b67e512fe", Secrets: "", CheckGPG: false},
		{Name: "libkcapi", Epoch: 0, Version: "1.3.1", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libkcapi-1.3.1-3.el9.x86_64.rpm", Checksum: "sha256:9b4733e8a790b51d845cedfa67e6321fd5a2923dd0fb7ce1f5e630aa382ba3c1", Secrets: "", CheckGPG: false},
		{Name: "libkcapi-hmaccalc", Epoch: 0, Version: "1.3.1", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libkcapi-hmaccalc-1.3.1-3.el9.x86_64.rpm", Checksum: "sha256:1b39f1faa4a8813cbfa2650e82f6ea06a4248e0c493ce7e4829c7d892e7757ed", Secrets: "", CheckGPG: false},
		{Name: "libmount", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libmount-2.37.2-1.el9.x86_64.rpm", Checksum: "sha256:26191af0cc7acf9bb335ebd8b4ed357582165ee3be78fce9f4395f84ad2805ce", Secrets: "", CheckGPG: false},
		{Name: "libpwquality", Epoch: 0, Version: "1.4.4", Release: "8.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libpwquality-1.4.4-8.el9.x86_64.rpm", Checksum: "sha256:93f00e5efac1e3f1ecbc0d6a4c068772cb12912cd20c9ea58716d6c0cd004886", Secrets: "", CheckGPG: false},
		{Name: "libseccomp", Epoch: 0, Version: "2.5.2", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libseccomp-2.5.2-2.el9.x86_64.rpm", Checksum: "sha256:d5c1c4473ebf5fd9c605eb866118d7428cdec9b188db18e45545801cc2a689c3", Secrets: "", CheckGPG: false},
		{Name: "libselinux", Epoch: 0, Version: "3.3", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libselinux-3.3-2.el9.x86_64.rpm", Checksum: "sha256:8e589b8408b04cbc19564620b229b6768edbaeb9090885d2273d84b8fc2f172b", Secrets: "", CheckGPG: false},
		{Name: "libsemanage", Epoch: 0, Version: "3.3", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libsemanage-3.3-1.el9.x86_64.rpm", Checksum: "sha256:7e62a0ed0a508486b565e48794a146022f344aeb6801834ad7c5afe6a97ef065", Secrets: "", CheckGPG: false},
		{Name: "libsepol", Epoch: 0, Version: "3.3", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libsepol-3.3-2.el9.x86_64.rpm", Checksum: "sha256:fc508147fe876706b61941a6ce554d7f7786f1ec3d097c4411fd6c7511acd289", Secrets: "", CheckGPG: false},
		{Name: "libsigsegv", Epoch: 0, Version: "2.13", Release: "4.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libsigsegv-2.13-4.el9.x86_64.rpm", Checksum: "sha256:931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a", Secrets: "", CheckGPG: false},
		{Name: "libsmartcols", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libsmartcols-2.37.2-1.el9.x86_64.rpm", Checksum: "sha256:c62433784604a2e6571e0fcbdd4a2d60f059c5c15624207998c5f03b18d9d382", Secrets: "", CheckGPG: false},
		{Name: "libtasn1", Epoch: 0, Version: "4.16.0", Release: "7.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libtasn1-4.16.0-7.el9.x86_64.rpm", Checksum: "sha256:656031558c53da4a5b3ccfd883bd6d55996037891323152b1f07e8d1d5377406", Secrets: "", CheckGPG: false},
		{Name: "libutempter", Epoch: 0, Version: "1.2.1", Release: "6.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libutempter-1.2.1-6.el9.x86_64.rpm", Checksum: "sha256:fab361a9cba04490fd8b5664049983d1e57ebf7c1080804726ba600708524125", Secrets: "", CheckGPG: false},
		{Name: "libuuid", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libuuid-2.37.2-1.el9.x86_64.rpm", Checksum: "sha256:ffd8317ccc6f80524b7bf15a8157d82f36a2b9c7478bb04eb4a34c18d019e6fa", Secrets: "", CheckGPG: false},
		{Name: "libxcrypt", Epoch: 0, Version: "4.4.18", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libxcrypt-4.4.18-3.el9.x86_64.rpm", Checksum: "sha256:97e88678b420f619a44608fff30062086aa1dd6931ecbd54f21bba005ff1de1a", Secrets: "", CheckGPG: false},
		{Name: "libzstd", Epoch: 0, Version: "1.5.0", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/libzstd-1.5.0-2.el9.x86_64.rpm", Checksum: "sha256:8282f33f06743ab88e36fea978559ac617c44cda14eb65495cad37505fdace41", Secrets: "", CheckGPG: false},
		{Name: "linux-firmware", Epoch: 0, Version: "20211216", Release: "124.el9", Arch: "noarch", RemoteLocation: "%s/Packages/linux-firmware-20211216-124.el9.noarch.rpm", Checksum: "sha256:0524c9cd96db4d57a5c190165ee2f8ade91854e21c19c61c6bd3504030ca24fa", Secrets: "", CheckGPG: false},
		{Name: "linux-firmware-whence", Epoch: 0, Version: "20211216", Release: "124.el9", Arch: "noarch", RemoteLocation: "%s/Packages/linux-firmware-whence-20211216-124.el9.noarch.rpm", Checksum: "sha256:7c58504c14979118ea36352982aaa5814ba0f448e17c1baddb811b9511315a58", Secrets: "", CheckGPG: false},
		{Name: "lz4-libs", Epoch: 0, Version: "1.9.3", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/lz4-libs-1.9.3-5.el9.x86_64.rpm", Checksum: "sha256:cba6a63054d070956a182e33269ee245bcfbe87e3e605c27816519db762a66ad", Secrets: "", CheckGPG: false},
		{Name: "ncurses-base", Epoch: 0, Version: "6.2", Release: "8.20210508.el9", Arch: "noarch", RemoteLocation: "%s/Packages/ncurses-base-6.2-8.20210508.el9.noarch.rpm", Checksum: "sha256:e4cc4a4a479b8c27776debba5c20e8ef21dc4b513da62a25ed09f88386ac08a8", Secrets: "", CheckGPG: false},
		{Name: "ncurses-libs", Epoch: 0, Version: "6.2", Release: "8.20210508.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/ncurses-libs-6.2-8.20210508.el9.x86_64.rpm", Checksum: "sha256:328f4d50e66b00f24344ebe239817204fda8e68b1d988c6943abb3c36231beaa", Secrets: "", CheckGPG: false},
		{Name: "openssl", Epoch: 1, Version: "3.0.1", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/openssl-3.0.1-5.el9.x86_64.rpm", Checksum: "sha256:aa9ee73fe806ddeafab4a5b0e370256e6c61f67f67114101d0735aed3c9a5eda", Secrets: "", CheckGPG: false},
		{Name: "openssl-libs", Epoch: 1, Version: "3.0.1", Release: "5.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm", Checksum: "sha256:fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666", Secrets: "", CheckGPG: false},
		{Name: "openssl-pkcs11", Epoch: 0, Version: "0.4.11", Release: "7.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/openssl-pkcs11-0.4.11-7.el9.x86_64.rpm", Checksum: "sha256:4be41142a5fb2b4cd6d812e126838cffa57b7c84e5a79d65f66bb9cf1d2830a3", Secrets: "", CheckGPG: false},
		{Name: "p11-kit", Epoch: 0, Version: "0.24.1", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/p11-kit-0.24.1-2.el9.x86_64.rpm", Checksum: "sha256:da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6", Secrets: "", CheckGPG: false},
		{Name: "p11-kit-trust", Epoch: 0, Version: "0.24.1", Release: "2.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/p11-kit-trust-0.24.1-2.el9.x86_64.rpm", Checksum: "sha256:ae9a633c58980328bef6358c6aa3c9ce0a65130c66fbfa4249922ddf5a3e2bb1", Secrets: "", CheckGPG: false},
		{Name: "pam", Epoch: 0, Version: "1.5.1", Release: "9.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/pam-1.5.1-9.el9.x86_64.rpm", Checksum: "sha256:e64caedce811645ecdd78e7b4ae83c189aa884ff1ba6445374f39186c588c52c", Secrets: "", CheckGPG: false},
		{Name: "pcre", Epoch: 0, Version: "8.44", Release: "3.el9.3", Arch: "x86_64", RemoteLocation: "%s/Packages/pcre-8.44-3.el9.3.x86_64.rpm", Checksum: "sha256:4a3cb61eb08c4f24e44756b6cb329812fe48d5c65c1fba546fadfa975045a8c5", Secrets: "", CheckGPG: false},
		{Name: "pcre2", Epoch: 0, Version: "10.37", Release: "3.el9.1", Arch: "x86_64", RemoteLocation: "%s/Packages/pcre2-10.37-3.el9.1.x86_64.rpm", Checksum: "sha256:441e71f24e95b7c319f02264db53f88aa49778b2214f7dd5c75f1a3838e72dea", Secrets: "", CheckGPG: false},
		{Name: "pcre2-syntax", Epoch: 0, Version: "10.37", Release: "3.el9.1", Arch: "noarch", RemoteLocation: "%s/Packages/pcre2-syntax-10.37-3.el9.1.noarch.rpm", Checksum: "sha256:55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e", Secrets: "", CheckGPG: false},
		{Name: "procps-ng", Epoch: 0, Version: "3.3.17", Release: "4.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/procps-ng-3.3.17-4.el9.x86_64.rpm", Checksum: "sha256:3a7cc3f6d6dfdaeb9e7bfdb06d968c3ae78246ff4f793c2d2e2bd71156961d69", Secrets: "", CheckGPG: false},
		{Name: "readline", Epoch: 0, Version: "8.1", Release: "4.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/readline-8.1-4.el9.x86_64.rpm", Checksum: "sha256:49945472925286ad89b0575657b43f9224777e36b442f0c88df67f0b61e26aee", Secrets: "", CheckGPG: false},
		{Name: "sed", Epoch: 0, Version: "4.8", Release: "9.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/sed-4.8-9.el9.x86_64.rpm", Checksum: "sha256:a2c5d9a7f569abb5a592df1c3aaff0441bf827c9d0e2df0ab42b6c443dbc475f", Secrets: "", CheckGPG: false},
		{Name: "setup", Epoch: 0, Version: "2.13.7", Release: "6.el9", Arch: "noarch", RemoteLocation: "%s/Packages/setup-2.13.7-6.el9.noarch.rpm", Checksum: "sha256:c0202712e8ec928cf61f3d777f23859ba6de2e85786e928ee5472fdde570aeee", Secrets: "", CheckGPG: false},
		{Name: "shadow-utils", Epoch: 2, Version: "4.9", Release: "3.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/shadow-utils-4.9-3.el9.x86_64.rpm", Checksum: "sha256:46fca2ed21478e5143434da4fbd47ca4599a885fab9f8636f9c7ba54942dd27e", Secrets: "", CheckGPG: false},
		{Name: "systemd", Epoch: 0, Version: "249", Release: "9.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/systemd-249-9.el9.x86_64.rpm", Checksum: "sha256:eb57c1242f8a7d68e6c258f40b048d8b7bd0749254ac97b7f399b3bc8011a81b", Secrets: "", CheckGPG: false},
		{Name: "systemd-libs", Epoch: 0, Version: "249", Release: "9.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/systemd-libs-249-9.el9.x86_64.rpm", Checksum: "sha256:708fbc3c7fd77a21e0b391e2a80d5c344962de9865e79514b2c89210ef06ba39", Secrets: "", CheckGPG: false},
		{Name: "systemd-pam", Epoch: 0, Version: "249", Release: "9.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/systemd-pam-249-9.el9.x86_64.rpm", Checksum: "sha256:eb7af981fb95425c68ccb0e4b95688672afd3032b57002e65fda8f734a089556", Secrets: "", CheckGPG: false},
		{Name: "systemd-rpm-macros", Epoch: 0, Version: "249", Release: "9.el9", Arch: "noarch", RemoteLocation: "%s/Packages/systemd-rpm-macros-249-9.el9.noarch.rpm", Checksum: "sha256:3552f7cc9077d5831f859f6cf721d419eccc83cb381d14a7a1483512272bd586", Secrets: "", CheckGPG: false},
		{Name: "systemd-udev", Epoch: 0, Version: "249", Release: "9.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/systemd-udev-249-9.el9.x86_64.rpm", Checksum: "sha256:d9c47e7088b8d279b8fd51e2274df957c3b68a265b42123ef9bbeb339d5ce3ba", Secrets: "", CheckGPG: false},
		{Name: "tmux", Epoch: 0, Version: "3.2a", Release: "4.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/tmux-3.2a-4.el9.x86_64.rpm", Checksum: "sha256:68074b673bfac39af1fbfc85d43bc1c111456db01a563bda6400ad485de5eb70", Secrets: "", CheckGPG: false},
		{Name: "tzdata", Epoch: 0, Version: "2021e", Release: "1.el9", Arch: "noarch", RemoteLocation: "%s/Packages/tzdata-2021e-1.el9.noarch.rpm", Checksum: "sha256:42d89577a0f887c4baa162250862dea2c1830b1ced56c45ced9645ad8e2a3671", Secrets: "", CheckGPG: false},
		{Name: "util-linux", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/util-linux-2.37.2-1.el9.x86_64.rpm", Checksum: "sha256:4ca41a925461daa936db284a59bf325ea061cdb39d8738e288cc19afe30a8ae8", Secrets: "", CheckGPG: false},
		{Name: "util-linux-core", Epoch: 0, Version: "2.37.2", Release: "1.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/util-linux-core-2.37.2-1.el9.x86_64.rpm", Checksum: "sha256:0313682867c1d07785a6d02ff87e1899f484bd1ce6348fa5c673eca78c0da2bd", Secrets: "", CheckGPG: false},
		{Name: "vim-minimal", Epoch: 2, Version: "8.2.2637", Release: "11.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/vim-minimal-8.2.2637-11.el9.x86_64.rpm", Checksum: "sha256:ab6e48c8a118bed88dc734aaf21e743b57e94d448f9e38745c3b777af96809c7", Secrets: "", CheckGPG: false},
		{Name: "xz", Epoch: 0, Version: "5.2.5", Release: "7.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/xz-5.2.5-7.el9.x86_64.rpm", Checksum: "sha256:b1c2d99961e50bb46400caa528aab9c7b361f5754427fd05ae22a7b551bf2ce5", Secrets: "", CheckGPG: false},
		{Name: "xz-libs", Epoch: 0, Version: "5.2.5", Release: "7.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/xz-libs-5.2.5-7.el9.x86_64.rpm", Checksum: "sha256:770819da28cce56e2e2b141b0eee1694d7f3dcf78a5700e1469436461399f001", Secrets: "", CheckGPG: false},
		{Name: "zlib", Epoch: 0, Version: "1.2.11", Release: "31.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/zlib-1.2.11-31.el9.x86_64.rpm", Checksum: "sha256:1c59b113fda8863e9066cc5d01c6d00bd9c50c4650e1c5b932082c8886e185d1", Secrets: "", CheckGPG: false},
		{Name: "zsh", Epoch: 0, Version: "5.8", Release: "7.el9", Arch: "x86_64", RemoteLocation: "%s/Packages/zsh-5.8-7.el9.x86_64.rpm", Checksum: "sha256:133da157fbd2b43e4a41af3ba7bb5267cf9ebed0aaf8124a76e5eca948c37572", Secrets: "", CheckGPG: false},
	}

	exp := []rpmmd.PackageSpec(expectedTemplate)
	for idx := range exp {
		urlTemplate := exp[idx].RemoteLocation
		exp[idx].RemoteLocation = fmt.Sprintf(urlTemplate, repo.BaseURL)
	}
	return exp
}

func TestErrorRepoInfo(t *testing.T) {
	if !*forceDNF {
		// dnf tests aren't forced: skip them if the dnf sniff check fails
		if !dnfInstalled() {
			t.Skip()
		}
	}

	assert := assert.New(t)

	type testCase struct {
		repo   rpmmd.RepoConfig
		expMsg string
	}

	testCases := []testCase{
		{
			repo: rpmmd.RepoConfig{
				Name:     "",
				BaseURL:  "https://0.0.0.0/baseos/repo",
				Metalink: "https://0.0.0.0/baseos/metalink",
			},
			expMsg: "[https://0.0.0.0/baseos/repo]",
		},
		{
			repo: rpmmd.RepoConfig{
				Name:     "baseos",
				BaseURL:  "https://0.0.0.0/baseos/repo",
				Metalink: "https://0.0.0.0/baseos/metalink",
			},
			expMsg: "[baseos: https://0.0.0.0/baseos/repo]",
		},
		{
			repo: rpmmd.RepoConfig{
				Name:     "fedora",
				Metalink: "https://0.0.0.0/f35/metalink",
			},
			expMsg: "[fedora: https://0.0.0.0/f35/metalink]",
		},
		{
			repo: rpmmd.RepoConfig{
				Name:       "",
				MirrorList: "https://0.0.0.0/baseos/mirrors",
			},
			expMsg: "[https://0.0.0.0/baseos/mirrors]",
		},
	}

	solver := NewSolver("f36", "36", "x86_64", "/tmp/cache")
	solver.SetDNFJSONPath("../../dnf-json")
	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			_, err := solver.Depsolve([]rpmmd.PackageSet{
				{
					Include:      []string{"osbuild"},
					Exclude:      nil,
					Repositories: []rpmmd.RepoConfig{tc.repo},
				},
			})
			assert.Error(err)
			assert.Contains(err.Error(), tc.expMsg)
		})
	}
}
