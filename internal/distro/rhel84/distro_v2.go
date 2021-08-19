package rhel84

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"

	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type imageTypeS2 struct {
	arch             *architecture
	name             string
	filename         string
	mimeType         string
	packageSets      map[string]rpmmd.PackageSet
	enabledServices  []string
	disabledServices []string
	defaultTarget    string
	kernelOptions    string
	bootable         bool
	bootISO          bool
	rpmOstree        bool
	defaultSize      uint64
}

func (t *imageTypeS2) Arch() distro.Arch {
	return t.arch
}

func (t *imageTypeS2) Name() string {
	return t.name
}

func (t *imageTypeS2) Filename() string {
	return t.filename
}

func (t *imageTypeS2) MIMEType() string {
	return t.mimeType
}

func (t *imageTypeS2) OSTreeRef() string {
	if t.rpmOstree {
		return fmt.Sprintf(ostreeRef, t.arch.name)
	}
	return ""
}

func (t *imageTypeS2) Size(size uint64) uint64 {
	const MegaByte = 1024 * 1024
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%MegaByte != 0 {
		size = (size/MegaByte + 1) * MegaByte
	}
	if size == 0 {
		size = t.defaultSize
	}
	return size
}

func (t *imageTypeS2) Packages(bp blueprint.Blueprint) ([]string, []string) {
	packages := append(t.packageSets["packages"].Include, bp.GetPackages()...)
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		packages = append(packages, "chrony")
	}

	// copy the list of excluded packages from the image type
	// and subtract any packages found in the blueprint (this
	// will not handle the issue with dependencies present in
	// the list of excluded packages, but it will create a
	// possibility of a workaround at least)
	excludedPackages := append([]string(nil), t.packageSets["packages"].Exclude...)
	for _, pkg := range bp.GetPackages() {
		// removePackage is fine if the package doesn't exist
		excludedPackages = removePackage(excludedPackages, pkg)
	}

	return packages, excludedPackages
}

func (t *imageTypeS2) BuildPackages() []string {
	buildPackages := append(t.arch.distro.buildPackages, t.arch.buildPackages...)
	if t.rpmOstree {
		buildPackages = append(buildPackages, "rpm-ostree")
	}
	if t.bootISO {
		buildPackages = append(buildPackages, t.packageSets["build"].Include...)
	}
	return buildPackages
}

func (t *imageTypeS2) PackageSets(bp blueprint.Blueprint) map[string]rpmmd.PackageSet {
	sets := map[string]rpmmd.PackageSet{
		"build-packages": {
			Include: t.BuildPackages(),
		},
	}
	for name, pkgSet := range t.packageSets {
		if name == "packages" {
			// treat base packages separately to combine with blueprint
			packages := new(rpmmd.PackageSet)
			packages.Include, packages.Exclude = t.Packages(bp)
			sets[name] = *packages
			continue
		}
		sets[name] = pkgSet
	}
	return sets
}

func (t *imageTypeS2) Exports() []string {
	return []string{"assembler"}
}

func (t *imageTypeS2) Manifest(c *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	seed int64) (distro.Manifest, error) {
	source := rand.NewSource(seed)
	rng := rand.New(source)
	pipelines, err := t.pipelines(c, options, repos, packageSpecSets, rng)
	if err != nil {
		return distro.Manifest{}, err
	}

	// flatten spec sets for sources
	allPackageSpecs := make([]rpmmd.PackageSpec, 0)
	for _, specs := range packageSpecSets {
		allPackageSpecs = append(allPackageSpecs, specs...)
	}

	var commits []ostreeCommit
	if t.bootISO && options.OSTree.Parent != "" && options.OSTree.URL != "" {
		commit := ostreeCommit{Checksum: options.OSTree.Parent, URL: options.OSTree.URL}
		commits = []ostreeCommit{commit}
	}
	return json.Marshal(
		osbuild.Manifest{
			Version:   "2",
			Pipelines: pipelines,
			Sources:   t.sources(allPackageSpecs, commits),
		},
	)
}

// local type for ostree commit metadata used to define commit sources
type ostreeCommit struct {
	Checksum string
	URL      string
}

func (t *imageTypeS2) sources(packages []rpmmd.PackageSpec, ostreeCommits []ostreeCommit) osbuild.Sources {
	sources := osbuild.Sources{}
	curl := &osbuild.CurlSource{
		Items: make(map[string]osbuild.CurlSourceItem),
	}
	for _, pkg := range packages {
		item := new(osbuild.URLWithSecrets)
		item.URL = pkg.RemoteLocation
		if pkg.Secrets == "org.osbuild.rhsm" {
			item.Secrets = &osbuild.URLSecrets{
				Name: "org.osbuild.rhsm",
			}
		}
		curl.Items[pkg.Checksum] = item
	}
	if len(curl.Items) > 0 {
		sources["org.osbuild.curl"] = curl
	}

	ostree := &osbuild.OSTreeSource{
		Items: make(map[string]osbuild.OSTreeSourceItem),
	}
	for _, commit := range ostreeCommits {
		item := new(osbuild.OSTreeSourceItem)
		item.Remote.URL = commit.URL
		ostree.Items[commit.Checksum] = *item
	}
	if len(ostree.Items) > 0 {
		sources["org.osbuild.ostree"] = ostree
	}
	return sources
}

func (t *imageTypeS2) pipelines(customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {

	if t.bootISO {
		if options.OSTree.Parent == "" {
			return nil, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}
		if customizations != nil {
			return nil, fmt.Errorf("boot ISO image type %q does not support blueprint customizations", t.name)
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return nil, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()

	if mountpoints != nil && t.rpmOstree {
		return nil, fmt.Errorf("Custom mountpoints are not supported for ostree types")
	}

	invalidMountpoints := []string{}
	for _, m := range mountpoints {
		if m.Mountpoint != "/" {
			invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
		}
	}

	if len(invalidMountpoints) > 0 {
		return nil, fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
	}

	pipelines := make([]osbuild.Pipeline, 0)

	pipelines = append(pipelines, *t.buildPipeline(repos, packageSetSpecs["build-packages"]))

	if t.bootISO {
		var kernelPkg rpmmd.PackageSpec
		for _, pkg := range packageSetSpecs["installer"] {
			if pkg.Name == "kernel" {
				kernelPkg = pkg
				break
			}
		}
		// TODO: panic if not found
		kernelVer := fmt.Sprintf("%s-%s.%s", kernelPkg.Version, kernelPkg.Release, kernelPkg.Arch)
		pipelines = append(pipelines, *t.anacondaTreePipeline(repos, packageSetSpecs["installer"], options, kernelVer))
		pipelines = append(pipelines, *t.bootISOTreePipeline(kernelVer))
		pipelines = append(pipelines, *t.bootISOPipeline())
	} else {
		treePipeline, err := t.ostreeTreePipeline(repos, packageSetSpecs["packages"], customizations)
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, *treePipeline)
		pipelines = append(pipelines, *t.ostreeCommitPipeline(options))
		pipelines = append(pipelines, *t.containerTreePipeline(repos, packageSetSpecs["container"], options, customizations))
		pipelines = append(pipelines, *t.containerPipeline())
	}

	return pipelines, nil
}

func (t *imageTypeS2) buildPipeline(repos []rpmmd.RepoConfig, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "build"
	p.Runner = "org.osbuild.rhel84"
	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(repos), t.rpmStageInputs(buildPackageSpecs)))
	p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))
	return p
}

func (t *imageTypeS2) ostreeTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, c *blueprint.Customizations) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	p.Name = "ostree-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(repos), t.rpmStageInputs(packages)))
	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))
	}
	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *keyboard}))
	}
	if hostname := c.GetHostname(); hostname != nil {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: *hostname}))
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: *timezone}))
	} else {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: "America/New_York"}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: ntpServers}))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(t.groupStageOptions(groups)))
	}

	if users := c.GetUsers(); len(users) > 0 {
		options, err := t.userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(osbuild.NewUsersStage(options))
		p.AddStage(osbuild.NewFirstBootStage(t.usersFirstBootOptions(options)))
	}

	if services := c.GetServices(); services != nil || t.enabledServices != nil || t.disabledServices != nil || t.defaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(t.systemdStageOptions(t.enabledServices, t.disabledServices, services, t.defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(t.firewallStageOptions(firewall)))
	}

	if !t.bootISO {
		p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))
	}

	// These are the current defaults for the sysconfig stage. This can be changed to be image type exclusive if different configs are needed.
	p.AddStage(osbuild.NewSysconfigStage(&osbuild.SysconfigStageOptions{
		Kernel: osbuild.SysconfigKernelOptions{
			UpdateDefault: true,
			DefaultKernel: "kernel",
		},
		Network: osbuild.SysconfigNetworkOptions{
			Networking: true,
			NoZeroConf: true,
		},
	}))

	p.AddStage(osbuild.NewOSTreePrepTreeStage(&osbuild.OSTreePrepTreeStageOptions{
		EtcGroupMembers: []string{
			// NOTE: We may want to make this configurable.
			"wheel", "docker",
		},
	}))
	return p, nil
}

func (t *imageTypeS2) ostreeCommitPipeline(options distro.ImageOptions) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "ostree-commit"
	p.Build = "name:build"
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: "/repo"}))

	commitStageInput := new(osbuild.OSTreeCommitStageInput)
	commitStageInput.Type = "org.osbuild.tree"
	commitStageInput.Origin = "org.osbuild.pipeline"
	commitStageInput.References = osbuild.OSTreeCommitStageReferences{"name:ostree-tree"}

	p.AddStage(osbuild.NewOSTreeCommitStage(
		&osbuild.OSTreeCommitStageOptions{
			Ref:       options.OSTree.Ref,
			OSVersion: "8.4", // NOTE: Set on image type?
			Parent:    options.OSTree.Parent,
		},
		&osbuild.OSTreeCommitStageInputs{Tree: commitStageInput}),
	)
	return p
}

func (t *imageTypeS2) containerTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, options distro.ImageOptions, c *blueprint.Customizations) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "container-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(repos), t.rpmStageInputs(packages)))
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US"}))
	}
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: "/var/www/html/repo"}))

	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: "/var/www/html/repo"},
		t.ostreePullStageInputs("org.osbuild.pipeline", "name:ostree-commit", options.OSTree.Ref),
	))
	return p
}

func (t *imageTypeS2) containerPipeline() *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	// NOTE(akoutsou) 1to2t: final pipeline should always be named "assembler"
	p.Name = "assembler"
	p.Build = "name:build"
	options := &osbuild.OCIArchiveStageOptions{
		Architecture: t.arch.Name(),
		Filename:     t.Filename(),
		Config: &osbuild.OCIArchiveConfig{
			Cmd:          []string{"httpd", "-D", "FOREGROUND"},
			ExposedPorts: []string{"80"},
		},
	}
	baseInput := new(osbuild.OCIArchiveStageInput)
	baseInput.Type = "org.osbuild.tree"
	baseInput.Origin = "org.osbuild.pipeline"
	baseInput.References = []string{"name:container-tree"}
	inputs := &osbuild.OCIArchiveStageInputs{Base: baseInput}
	p.AddStage(osbuild.NewOCIArchiveStage(options, inputs))
	return p
}

func (t *imageTypeS2) anacondaTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, options distro.ImageOptions, kernelVer string) *osbuild.Pipeline {
	ostreeRepoPath := "/ostree/repo"
	p := new(osbuild.Pipeline)
	p.Name = "anaconda-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(repos), t.rpmStageInputs(packages)))
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: ostreeRepoPath}))
	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: ostreeRepoPath},
		t.ostreePullStageInputs("org.osbuild.source", options.OSTree.Parent, options.OSTree.Ref),
	))
	p.AddStage(osbuild.NewBuildstampStage(t.buildStampStageOptions()))
	p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))

	rootPassword := ""
	rootUser := osbuild.UsersStageOptionsUser{
		Password: &rootPassword,
	}

	installUID := 0
	installGID := 0
	installHome := "/root"
	installShell := "/usr/libexec/anaconda/run-anaconda"
	installPassword := ""
	installUser := osbuild.UsersStageOptionsUser{
		UID:      &installUID,
		GID:      &installGID,
		Home:     &installHome,
		Shell:    &installShell,
		Password: &installPassword,
	}
	usersStageOptions := &osbuild.UsersStageOptions{
		Users: map[string]osbuild.UsersStageOptionsUser{
			"root":    rootUser,
			"install": installUser,
		},
	}

	p.AddStage(osbuild.NewUsersStage(usersStageOptions))
	p.AddStage(osbuild.NewAnacondaStage(t.anacondaStageOptions()))
	p.AddStage(osbuild.NewLoraxScriptStage(t.loraxScriptStageOptions()))
	p.AddStage(osbuild.NewDracutStage(t.dracutStageOptions(kernelVer)))
	p.AddStage(osbuild.NewKickstartStage(t.kickstartStageOptions(fmt.Sprintf("file://%s", ostreeRepoPath), options.OSTree.Ref)))

	return p
}

func (t *imageTypeS2) bootISOTreePipeline(kernelVer string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso-tree"
	p.Build = "name:build"

	p.AddStage(osbuild.NewBootISOMonoStage(t.bootISOMonoStageOptions(kernelVer), t.bootISOMonoStageInputs()))
	p.AddStage(osbuild.NewDiscinfoStage(t.discinfoStageOptions()))

	return p
}
func (t *imageTypeS2) bootISOPipeline() *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	// NOTE(akoutsou) 1to2t: final pipeline should always be named "assembler"
	p.Name = "assembler"
	p.Build = "name:build"

	p.AddStage(osbuild.NewXorrisofsStage(t.xorrisofsStageOptions(), t.xorrisofsStageInputs()))
	p.AddStage(osbuild.NewImplantisomd5Stage(&osbuild.Implantisomd5StageOptions{Filename: t.Filename()}))

	return p
}

func (t *imageTypeS2) rpmStageInputs(specs []rpmmd.PackageSpec) *osbuild.RPMStageInputs {
	stageInput := new(osbuild.RPMStageInput)
	stageInput.Type = "org.osbuild.files"
	stageInput.Origin = "org.osbuild.source"
	stageInput.References = pkgRefs(specs)
	return &osbuild.RPMStageInputs{Packages: stageInput}
}

func pkgRefs(specs []rpmmd.PackageSpec) osbuild.RPMStageReferences {
	refs := make([]string, len(specs))
	for idx, pkg := range specs {
		refs[idx] = pkg.Checksum
	}
	return refs
}

func (t *imageTypeS2) ostreePullStageInputs(origin, source, commitRef string) *osbuild.OSTreePullStageInputs {
	pullStageInput := new(osbuild.OSTreePullStageInput)
	pullStageInput.Type = "org.osbuild.ostree"
	pullStageInput.Origin = origin

	inputRefs := make(map[string]osbuild.OSTreePullStageReference)
	inputRefs[source] = osbuild.OSTreePullStageReference{Ref: commitRef}
	pullStageInput.References = inputRefs
	return &osbuild.OSTreePullStageInputs{Commits: pullStageInput}
}

func (t *imageTypeS2) rpmStageOptions(repos []rpmmd.RepoConfig) *osbuild.RPMStageOptions {
	var gpgKeys []string
	for _, repo := range repos {
		if repo.GPGKey == "" {
			continue
		}
		gpgKeys = append(gpgKeys, repo.GPGKey)
	}

	return &osbuild.RPMStageOptions{
		GPGKeys: gpgKeys,
		Exclude: &osbuild.Exclude{
			// NOTE: Make configurable?
			Docs: true,
		},
	}
}

func (t *imageTypeS2) selinuxStageOptions() *osbuild.SELinuxStageOptions {

	options := &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
	if t.bootISO {
		options.Labels = map[string]string{
			"/usr/bin/cp":  "system_u:object_r:install_exec_t:s0",
			"/usr/bin/tar": "system_u:object_r:install_exec_t:s0",
		}
	}
	return options
}

func (t *imageTypeS2) userStageOptions(users []blueprint.UserCustomization) (*osbuild.UsersStageOptions, error) {
	options := osbuild.UsersStageOptions{
		Users: make(map[string]osbuild.UsersStageOptionsUser),
	}

	for _, c := range users {
		if c.Password != nil && !crypt.PasswordIsCrypted(*c.Password) {
			cryptedPassword, err := crypt.CryptSHA512(*c.Password)
			if err != nil {
				return nil, err
			}

			c.Password = &cryptedPassword
		}

		user := osbuild.UsersStageOptionsUser{
			Groups:      c.Groups,
			Description: c.Description,
			Home:        c.Home,
			Shell:       c.Shell,
			Password:    c.Password,
			Key:         c.Key,
		}

		user.UID = c.UID
		user.GID = c.GID

		options.Users[c.Name] = user
	}

	return &options, nil
}

func (t *imageTypeS2) usersFirstBootOptions(usersStageOptions *osbuild.UsersStageOptions) *osbuild.FirstBootStageOptions {
	cmds := make([]string, 0, 3*len(usersStageOptions.Users)+1)
	// workaround for creating authorized_keys file for user
	varhome := filepath.Join("/var", "home")
	for name, user := range usersStageOptions.Users {
		if user.Key != nil {
			sshdir := filepath.Join(varhome, name, ".ssh")
			cmds = append(cmds, fmt.Sprintf("mkdir -p %s", sshdir))
			cmds = append(cmds, fmt.Sprintf("sh -c 'echo %q >> %q'", *user.Key, filepath.Join(sshdir, "authorized_keys")))
			cmds = append(cmds, fmt.Sprintf("chown %s:%s -Rc %s", name, name, sshdir))
		}
	}
	cmds = append(cmds, fmt.Sprintf("restorecon -rvF %s", varhome))
	options := &osbuild.FirstBootStageOptions{
		Commands:       cmds,
		WaitForNetwork: false,
	}

	return options
}

func (t *imageTypeS2) groupStageOptions(groups []blueprint.GroupCustomization) *osbuild.GroupsStageOptions {
	options := osbuild.GroupsStageOptions{
		Groups: map[string]osbuild.GroupsStageOptionsGroup{},
	}

	for _, group := range groups {
		groupData := osbuild.GroupsStageOptionsGroup{
			Name: group.Name,
		}
		groupData.GID = group.GID

		options.Groups[group.Name] = groupData
	}

	return &options
}

func (t *imageTypeS2) firewallStageOptions(firewall *blueprint.FirewallCustomization) *osbuild.FirewallStageOptions {
	options := osbuild.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (t *imageTypeS2) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization, target string) *osbuild.SystemdStageOptions {
	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		disabledServices = append(disabledServices, s.Disabled...)
	}
	return &osbuild.SystemdStageOptions{
		EnabledServices:  enabledServices,
		DisabledServices: disabledServices,
		DefaultTarget:    target,
	}
}

func (t *imageTypeS2) buildStampStageOptions() *osbuild.BuildstampStageOptions {
	return &osbuild.BuildstampStageOptions{
		Arch:    t.Arch().Name(),
		Product: "Red Hat Enterprise Linux",
		Version: "8.4",
		Variant: "edge",
		Final:   true,
	}
}

func (t *imageTypeS2) anacondaStageOptions() *osbuild.AnacondaStageOptions {
	return &osbuild.AnacondaStageOptions{
		KickstartModules: []string{
			"org.fedoraproject.Anaconda.Modules.Network",
			"org.fedoraproject.Anaconda.Modules.Payloads",
			"org.fedoraproject.Anaconda.Modules.Storage",
		},
	}
}

func (t *imageTypeS2) loraxScriptStageOptions() *osbuild.LoraxScriptStageOptions {
	return &osbuild.LoraxScriptStageOptions{
		Path:     "99-generic/runtime-postinstall.tmpl",
		BaseArch: t.Arch().Name(),
	}
}

func (t *imageTypeS2) dracutStageOptions(kernelVer string) *osbuild.DracutStageOptions {
	kernel := []string{kernelVer}
	modules := []string{
		"bash",
		"systemd",
		"fips",
		"systemd-initrd",
		"modsign",
		"nss-softokn",
		"rdma",
		"rngd",
		"i18n",
		"convertfs",
		"network-manager",
		"network",
		"ifcfg",
		"url-lib",
		"drm",
		"plymouth",
		"prefixdevname",
		"prefixdevname-tools",
		"anaconda",
		"crypt",
		"dm",
		"dmsquash-live",
		"kernel-modules",
		"kernel-modules-extra",
		"kernel-network-modules",
		"livenet",
		"lvm",
		"mdraid",
		"multipath",
		"qemu",
		"qemu-net",
		"fcoe",
		"fcoe-uefi",
		"iscsi",
		"lunmask",
		"nfs",
		"resume",
		"rootfs-block",
		"terminfo",
		"udev-rules",
		"biosdevname",
		"dracut-systemd",
		"pollcdrom",
		"usrmount",
		"base",
		"fs-lib",
		"img-lib",
		"shutdown",
		"uefi-lib",
	}
	return &osbuild.DracutStageOptions{
		Kernel:  kernel,
		Modules: modules,
		Install: []string{"/.buildstamp"},
	}
}

func (t *imageTypeS2) kickstartStageOptions(ostreeURL, ostreeRef string) *osbuild.KickstartStageOptions {
	return &osbuild.KickstartStageOptions{
		Path: "/usr/share/anaconda/interactive-defaults.ks",
		OSTree: &osbuild.OSTreeOptions{
			OSName: "rhel",
			URL:    ostreeURL,
			Ref:    ostreeRef,
			GPG:    false,
		},
	}
}

func (t *imageTypeS2) bootISOMonoStageOptions(kernelVer string) *osbuild.BootISOMonoStageOptions {
	return &osbuild.BootISOMonoStageOptions{
		Product: osbuild.Product{
			Name:    "Red Hat Enterprise Linux",
			Version: "8.4",
		},
		ISOLabel: fmt.Sprintf("RHEL-8-4-0-BaseOS-%s", t.Arch().Name()),
		Kernel:   kernelVer,
		EFI: osbuild.EFI{
			Architectures: []string{
				"IA32",
				"X64",
			},
			Vendor: "redhat",
		},
		ISOLinux: osbuild.ISOLinux{
			Enabled: true,
			Debug:   false,
		},
		Templates: "80-rhel",
		RootFS: osbuild.RootFS{
			Size: 4096,
			Compression: osbuild.FSCompression{
				Method: "xz",
				Options: &osbuild.FSCompressionOptions{
					// TODO: based on image arch
					BCJ: "x86",
				},
			},
		},
	}
}

func (t *imageTypeS2) bootISOMonoStageInputs() *osbuild.BootISOMonoStageInputs {
	rootfsInput := new(osbuild.BootISOMonoStageInput)
	rootfsInput.Type = "org.osbuild.tree"
	rootfsInput.Origin = "org.osbuild.pipeline"
	rootfsInput.References = osbuild.BootISOMonoStageReferences{"name:anaconda-tree"}
	return &osbuild.BootISOMonoStageInputs{
		RootFS: rootfsInput,
	}
}

func (t *imageTypeS2) discinfoStageOptions() *osbuild.DiscinfoStageOptions {
	return &osbuild.DiscinfoStageOptions{
		BaseArch: t.Arch().Name(),
		Release:  "202010217.n.0",
	}
}

func (t *imageTypeS2) xorrisofsStageOptions() *osbuild.XorrisofsStageOptions {
	return &osbuild.XorrisofsStageOptions{
		Filename: t.Filename(),
		VolID:    fmt.Sprintf("RHEL-8-4-0-BaseOS-%s", t.Arch().Name()),
		SysID:    "LINUX",
		Boot: &osbuild.XorrisofsBoot{
			Image:   "isolinux/isolinux.bin",
			Catalog: "isolinux/boot.cat",
		},
		EFI:          "images/efiboot.img",
		IsohybridMBR: "/usr/share/syslinux/isohdpfx.bin",
	}
}

func (t *imageTypeS2) xorrisofsStageInputs() *osbuild.XorrisofsStageInputs {
	input := new(osbuild.XorrisofsStageInput)
	input.Type = "org.osbuild.tree"
	input.Origin = "org.osbuild.pipeline"
	input.References = osbuild.XorrisofsStageReferences{"name:bootiso-tree"}
	return &osbuild.XorrisofsStageInputs{Tree: input}
}
