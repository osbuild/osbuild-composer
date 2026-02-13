package osbuild

import (
	"fmt"
	"slices"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/rpmmd"
)

type RPMStageOptions struct {
	// Use the given path as RPM database
	DBPath string `json:"dbpath,omitempty"`

	// Array of GPG key contents to import
	GPGKeys []string `json:"gpgkeys,omitempty"`

	// Array of files in the tree containing GPG keys to import
	GPGKeysFromTree []string `json:"gpgkeys.fromtree,omitempty"`

	// Prevent dracut from running
	DisableDracut bool `json:"disable_dracut,omitempty"`

	Exclude *Exclude `json:"exclude,omitempty"`

	// Create the '/run/ostree-booted' marker
	OSTreeBooted *bool `json:"ostree_booted,omitempty"`

	// Set environment variables understood by kernel-install and plugins (kernel-install(8))
	KernelInstallEnv *KernelInstallEnv `json:"kernel_install_env,omitempty"`

	// Only install certain locales (sets `_install_langs` RPM macro)
	InstallLangs []string `json:"install_langs,omitempty"`

	RPMKeys *RPMKeys `json:"rpmkeys,omitempty"`
}

func (o *RPMStageOptions) Clone() *RPMStageOptions {
	if o == nil {
		return nil
	}

	return &RPMStageOptions{
		DBPath:           o.DBPath,
		GPGKeys:          slices.Clone(o.GPGKeys),
		GPGKeysFromTree:  slices.Clone(o.GPGKeysFromTree),
		DisableDracut:    o.DisableDracut,
		Exclude:          common.ClonePtr(o.Exclude),
		OSTreeBooted:     common.ClonePtr(o.OSTreeBooted),
		KernelInstallEnv: common.ClonePtr(o.KernelInstallEnv),
		InstallLangs:     slices.Clone(o.InstallLangs),
		RPMKeys:          common.ClonePtr(o.RPMKeys),
	}
}

type Exclude struct {
	// Do not install documentation.
	Docs bool `json:"docs,omitempty"`
}

type KernelInstallEnv struct {
	// Sets $BOOT_ROOT for kernel-install to override
	// $KERNEL_INSTALL_BOOT_ROOT, the installation location for boot entries
	BootRoot string `json:"boot_root,omitempty"`
}

type RPMKeys struct {
	BinPath              string `json:"bin_path,omitempty"`
	IgnoreImportFailures bool   `json:"ignore_import_failures,omitempty"`
}

// RPMPackage represents one RPM, as referenced by its content hash
// (checksum). The files source must indicate where to fetch the given
// RPM. If CheckGPG is `true` the RPM must be signed with one of the
// GPGKeys given in the RPMStageOptions.
type RPMPackage struct {
	Checksum string `json:"checksum"`
	CheckGPG bool   `json:"check_gpg,omitempty"`
}

func (RPMStageOptions) isStageOptions() {}

// RPMStageInputs defines a collection of packages to be installed by the RPM
// stage.
type RPMStageInputs struct {
	// Packages to install
	Packages *FilesInput `json:"packages"`
}

func (RPMStageInputs) isStageInputs() {}

// RPM package reference metadata
type RPMStageReferenceMetadata struct {
	// This option defaults to `false`, therefore it does not need to be
	// defined as pointer to bool and can be omitted.
	CheckGPG bool `json:"rpm.check_gpg,omitempty"`
}

func (*RPMStageReferenceMetadata) isFilesInputRefMetadata() {}

// NewRPMStage creates a new RPM stage.
func NewRPMStage(options *RPMStageOptions, inputs *RPMStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.rpm",
		Inputs:  inputs,
		Options: options,
	}
}

// RPMStageMetadata gives the set of packages installed by the RPM stage
type RPMStageMetadata struct {
	Packages []RPMPackageMetadata `json:"packages"`
}

// RPMPackageMetadata contains the metadata extracted from one RPM header
type RPMPackageMetadata struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Release string  `json:"release"`
	Epoch   *string `json:"epoch"`
	Arch    string  `json:"arch"`
	SigMD5  string  `json:"sigmd5"`
	SigPGP  string  `json:"sigpgp"`
	SigGPG  string  `json:"siggpg"`
}

func (pkgmd RPMPackageMetadata) Signature() *string {
	if pkgmd.SigGPG != "" {
		return &pkgmd.SigGPG
	} else if pkgmd.SigPGP != "" {
		return &pkgmd.SigPGP
	}
	return nil
}

func (RPMStageMetadata) isStageMetadata() {}

func NewRpmStageSourceFilesInputs(pkgs rpmmd.PackageList) *RPMStageInputs {
	input := NewFilesInput(pkgRefs(pkgs))
	return &RPMStageInputs{Packages: input}
}

func pkgRefs(pkgs rpmmd.PackageList) FilesInputRef {
	refs := make([]FilesInputSourceArrayRefEntry, len(pkgs))
	for idx, pkg := range pkgs {
		var pkgMetadata FilesInputRefMetadata
		if pkg.CheckGPG {
			pkgMetadata = &RPMStageReferenceMetadata{
				CheckGPG: pkg.CheckGPG,
			}
		}
		refs[idx] = NewFilesInputSourceArrayRefEntry(pkg.Checksum.String(), pkgMetadata)
	}
	return NewFilesInputSourceArrayRef(refs)
}

// GPGKeysForPackages collects and returns deduplicated GPG keys from the
// repositories that the given packages come from. This is used to import
// only the GPG keys needed for a specific set of packages, rather than
// importing all keys from all configured repositories.
//
// Returns an error if:
// - Any package has a nil Repo pointer (indicates a bug in depsolving)
// - Any package requires GPG checking but its repo has no GPG keys configured
//
// The returned keys are sorted for deterministic output.
//
// NOTE: Currently collects keys even for packages/repos with CheckGPG=false.
// This could be changed if importing unused keys is not desirable.
func GPGKeysForPackages(pkgs rpmmd.PackageList) ([]string, error) {
	keyMap := make(map[string]bool)
	var gpgKeys []string
	for _, pkg := range pkgs {
		if pkg.Repo == nil {
			return nil, fmt.Errorf("package %q has nil Repo pointer. This is a bug in depsolving.", pkg.Name)
		}
		if pkg.CheckGPG && len(pkg.Repo.GPGKeys) == 0 {
			return nil, fmt.Errorf(
				"package %q requires GPG check but repo %q has no GPG keys configured",
				pkg.Name, pkg.Repo.Id)
		}
		for _, key := range pkg.Repo.GPGKeys {
			if !keyMap[key] {
				gpgKeys = append(gpgKeys, key)
				keyMap[key] = true
			}
		}
	}
	slices.Sort(gpgKeys)
	return gpgKeys, nil
}

// GenRPMStagesFromTransactions creates RPM stages for each transaction.
// Each stage installs only the packages in its transaction and imports
// only the GPG keys needed for those packages.
//
// The baseOpts parameter provides template options that are copied to each
// stage. GPGKeys will be computed per transaction from package repos.
// GPGKeysFromTree will be filtered per transaction based on when the
// providing package is installed.
//
// Returns an error if any GPGKeysFromTree path is not provided by any package
// in the transactions.
func GenRPMStagesFromTransactions(
	transactions depsolvednf.TransactionList,
	baseOpts *RPMStageOptions,
) ([]*Stage, error) {
	if len(transactions) == 0 {
		return nil, nil
	}

	if baseOpts == nil {
		baseOpts = &RPMStageOptions{}
	}

	// Lookup which GPGKeysFromTree files come from which transaction
	var fileInfos map[string]depsolvednf.TransactionFileInfo
	if len(baseOpts.GPGKeysFromTree) > 0 {
		fileInfos = transactions.GetFilesTransactionInfo(baseOpts.GPGKeysFromTree)
		// Validate all paths are provided by some package in the transactions
		for _, keyPath := range baseOpts.GPGKeysFromTree {
			if _, found := fileInfos[keyPath]; !found {
				return nil, fmt.Errorf(
					"GPGKeysFromTree path %q not provided by any package in the transactions", keyPath)
			}
		}
	}

	stages := make([]*Stage, 0, len(transactions))
	for txIdx, pkgs := range transactions {
		if len(pkgs) == 0 {
			continue
		}

		opts := baseOpts.Clone()
		// Set repo-specific GPGKeys for this transaction's packages repos
		var err error
		opts.GPGKeys, err = GPGKeysForPackages(pkgs)
		if err != nil {
			return nil, err
		}
		// Filter GPGKeysFromTree for this transaction
		// NOTE: We need to reset the GPGKeysFromTree slice to avoid accumulating
		// keys from previous transactions or keeping the original slice from the base options.
		opts.GPGKeysFromTree = nil
		for _, keyPath := range baseOpts.GPGKeysFromTree {
			if info, found := fileInfos[keyPath]; !found || info.TxIndex != txIdx {
				continue
			}
			opts.GPGKeysFromTree = append(opts.GPGKeysFromTree, keyPath)
		}

		stages = append(stages, NewRPMStage(opts, NewRpmStageSourceFilesInputs(pkgs)))
	}
	return stages, nil
}
