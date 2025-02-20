// Standalone executable for generating all test manifests in parallel.
// Collects list of image types from the distro list.  Must be run from the
// root of the repository and reads tools/test-case-generators/repos.json for
// repositories tools/test-case-generators/format-request-map.json for
// customizations Collects errors and failures and prints them after all jobs
// are finished.

package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rhsm/facts"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"

	"github.com/osbuild/images/pkg/dnfjson"
)

type multiValue []string

func (mv *multiValue) String() string {
	return strings.Join(*mv, ", ")
}

func (mv *multiValue) Set(v string) error {
	split := strings.Split(v, ",")
	*mv = split
	return nil
}

type repository struct {
	Name           string   `json:"name"`
	Id             string   `json:"id,omitempty"`
	BaseURL        string   `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKey         string   `json:"gpgkey,omitempty"`
	CheckGPG       bool     `json:"check_gpg,omitempty"`
	CheckRepoGPG   bool     `json:"check_repo_gpg,omitempty"`
	IgnoreSSL      bool     `json:"ignore_ssl,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
	PackageSets    []string `json:"package-sets,omitempty"`
}

type ostreeOptions struct {
	Ref    string `json:"ref"`
	URL    string `json:"url"`
	Parent string `json:"parent"`
	RHSM   bool   `json:"rhsm"`
}

type crBlueprint struct {
	Name           string                    `json:"name,omitempty"`
	Description    string                    `json:"description,omitempty"`
	Version        string                    `json:"version,omitempty"`
	Packages       []blueprint.Package       `json:"packages,omitempty"`
	Modules        []blueprint.Package       `json:"modules,omitempty"`
	Groups         []blueprint.Group         `json:"groups,omitempty"`
	Containers     []blueprint.Container     `json:"containers,omitempty"`
	Customizations *blueprint.Customizations `json:"customizations,omitempty"`
	Distro         string                    `json:"distro,omitempty"`

	// EXPERIMENTAL
	Minimal bool `json:"minimal,omitempty"`
}

type composeRequest struct {
	Distro       string         `json:"distro,omitempty"`
	Arch         string         `json:"arch,omitempty"`
	ImageType    string         `json:"image-type,omitempty"`
	Repositories []repository   `json:"repositories,omitempty"`
	Filename     string         `json:"filename,omitempty"`
	OSTree       *ostreeOptions `json:"ostree,omitempty"`
	Blueprint    *crBlueprint   `json:"blueprint,omitempty"`
}

type manifestRequest struct {
	ComposeRequest  composeRequest            `json:"compose-request"`
	Overrides       map[string]composeRequest `json:"overrides"`
	SupportedArches []string                  `json:"supported_arches"`
}

type formatRequestMap map[string]manifestRequest

func loadFormatRequestMap() formatRequestMap {
	requestMapPath := "./tools/test-case-generators/format-request-map.json"
	fp, err := os.Open(requestMapPath)
	if err != nil {
		panic(fmt.Sprintf("failed to open format request map %q: %s", requestMapPath, err.Error()))
	}
	defer fp.Close()
	data, err := io.ReadAll(fp)
	if err != nil {
		panic(fmt.Sprintf("failed to read format request map %q: %s", requestMapPath, err.Error()))
	}
	var frm formatRequestMap
	if err := json.Unmarshal(data, &frm); err != nil {
		panic(fmt.Sprintf("failed to unmarshal format request map %q: %s", requestMapPath, err.Error()))
	}

	return frm
}

type manifestJob func(chan string) error

func makeManifestJob(name string, imgType distro.ImageType, cr composeRequest, distribution distro.Distro, archName string, seedArg int64, path string, cacheRoot string) manifestJob {
	distroName := distribution.Name()
	u := func(s string) string {
		return strings.Replace(s, "-", "_", -1)
	}
	filename := fmt.Sprintf("%s-%s-%s-boot.json", u(distroName), u(archName), u(name))
	cacheDir := filepath.Join(cacheRoot, archName+distribution.Name())

	options := distro.ImageOptions{Size: 0}
	if cr.OSTree != nil {
		options.OSTree = &ostree.ImageOptions{
			URL:       cr.OSTree.URL,
			ImageRef:  cr.OSTree.Ref,
			ParentRef: cr.OSTree.Parent,
			RHSM:      cr.OSTree.RHSM,
		}
	}

	// add RHSM fact to detect changes
	options.Facts = &facts.ImageOptions{
		APIType: facts.TEST_APITYPE,
	}

	job := func(msgq chan string) (err error) {
		defer func() {
			msg := fmt.Sprintf("Finished job %s", filename)
			if err != nil {
				msg += " [failed]"
			}
			msgq <- msg
		}()
		msgq <- fmt.Sprintf("Starting job %s", filename)
		repos := convertRepos(cr.Repositories)
		var bp blueprint.Blueprint
		if cr.Blueprint != nil {
			bp = blueprint.Blueprint(*cr.Blueprint)
			if bp.Minimal {
				msgq <- fmt.Sprintf("[%s] blueprint contains minimal=true, this is considered EXPERIMENTAL", filename)
			}
		}

		manifest, _, err := imgType.Manifest(&bp, options, repos, &seedArg)
		if err != nil {
			err = fmt.Errorf("[%s] failed: %s", filename, err)
			return
		}

		depsolved, err := depsolve(cacheDir, manifest.GetPackageSetChains(), distribution, archName)
		if err != nil {
			err = fmt.Errorf("[%s] depsolve failed: %s", filename, err.Error())
			return
		}
		if depsolved == nil {
			err = fmt.Errorf("[%s] nil package specs", filename)
			return
		}

		if cr.Blueprint != nil {
			bp = blueprint.Blueprint(*cr.Blueprint)
		}

		containerSpecs, err := resolvePipelineContainers(manifest.GetContainerSourceSpecs(), archName)
		if err != nil {
			return fmt.Errorf("[%s] container resolution failed: %s", filename, err.Error())
		}

		commitSpecs := resolvePipelineCommits(manifest.GetOSTreeSourceSpecs())

		mf, err := manifest.Serialize(depsolved, containerSpecs, commitSpecs, nil)
		if err != nil {
			return fmt.Errorf("[%s] manifest serialization failed: %s", filename, err.Error())
		}

		request := composeRequest{
			Distro:       distribution.Name(),
			Arch:         archName,
			ImageType:    cr.ImageType,
			Repositories: cr.Repositories,
			Filename:     cr.Filename,
			Blueprint:    cr.Blueprint,
			OSTree:       cr.OSTree,
		}
		rpmmd := map[string][]rpmmd.PackageSpec{}
		for plName, pkgSet := range depsolved {
			rpmmd[plName] = pkgSet.Packages
		}
		err = save(mf, rpmmd, containerSpecs, commitSpecs, request, path, filename)
		return
	}
	return job
}

type DistroArchRepoMap map[string]map[string][]repository

func (darm DistroArchRepoMap) ListDistros() []string {
	distros := make([]string, 0, len(darm))
	for d := range darm {
		distros = append(distros, d)
	}
	sort.Strings(distros)
	return distros
}

func convertRepo(r repository) rpmmd.RepoConfig {
	var urls []string
	if r.BaseURL != "" {
		urls = []string{r.BaseURL}
	}

	var keys []string
	if r.GPGKey != "" {
		keys = []string{r.GPGKey}
	}

	return rpmmd.RepoConfig{
		Id:             r.Id,
		Name:           r.Name,
		BaseURLs:       urls,
		Metalink:       r.Metalink,
		MirrorList:     r.MirrorList,
		GPGKeys:        keys,
		CheckGPG:       &r.CheckGPG,
		CheckRepoGPG:   &r.CheckRepoGPG,
		IgnoreSSL:      &r.IgnoreSSL,
		MetadataExpire: r.MetadataExpire,
		RHSM:           r.RHSM,
		ImageTypeTags:  r.ImageTypeTags,
		PackageSets:    r.PackageSets,
	}
}

func convertRepos(rr []repository) []rpmmd.RepoConfig {
	cr := make([]rpmmd.RepoConfig, len(rr))
	for idx, r := range rr {
		cr[idx] = convertRepo(r)
	}
	return cr
}

func readRepos() DistroArchRepoMap {
	file := "./tools/test-case-generators/repos.json"
	var darm DistroArchRepoMap
	fp, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer fp.Close()
	data, err := io.ReadAll(fp)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &darm); err != nil {
		panic(err)
	}
	return darm
}

func resolveContainers(containers []container.SourceSpec, archName string) ([]container.Spec, error) {
	resolver := container.NewResolver(archName)

	for _, c := range containers {
		resolver.Add(c)
	}

	return resolver.Finish()
}

func resolvePipelineContainers(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error) {
	containerSpecs := make(map[string][]container.Spec, len(containerSources))
	for plName, sourceSpecs := range containerSources {
		specs, err := resolveContainers(sourceSpecs, archName)
		if err != nil {
			return nil, err
		}
		containerSpecs[plName] = specs
	}
	return containerSpecs, nil
}

func resolveCommit(commitSource ostree.SourceSpec) ostree.CommitSpec {
	// "resolve" ostree commits by hashing the URL + ref to create a
	// realistic-looking commit ID in a deterministic way
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(commitSource.URL+commitSource.Ref)))
	spec := ostree.CommitSpec{
		Ref:      commitSource.Ref,
		URL:      commitSource.URL,
		Checksum: checksum,
	}
	if commitSource.RHSM {
		spec.Secrets = "org.osbuild.rhsm.consumer"
	}
	return spec
}

func resolvePipelineCommits(commitSources map[string][]ostree.SourceSpec) map[string][]ostree.CommitSpec {
	commits := make(map[string][]ostree.CommitSpec, len(commitSources))
	for name, commitSources := range commitSources {
		commitSpecs := make([]ostree.CommitSpec, len(commitSources))
		for idx, commitSource := range commitSources {
			commitSpecs[idx] = resolveCommit(commitSource)
		}
		commits[name] = commitSpecs
	}
	return commits
}

func depsolve(cacheDir string, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]dnfjson.DepsolveResult, error) {
	solver := dnfjson.NewSolver(d.ModulePlatformID(), d.Releasever(), arch, d.Name(), cacheDir)
	depsolvedSets := make(map[string]dnfjson.DepsolveResult)
	for name, pkgSet := range packageSets {
		res, err := solver.Depsolve(pkgSet, sbom.StandardTypeNone)
		if err != nil {
			return nil, err
		}
		depsolvedSets[name] = *res
	}
	return depsolvedSets, nil
}

func save(ms manifest.OSBuildManifest, pkgs map[string][]rpmmd.PackageSpec, containers map[string][]container.Spec, commits map[string][]ostree.CommitSpec, cr composeRequest, path, filename string) error {
	data := struct {
		ComposeRequest composeRequest                 `json:"compose-request"`
		Manifest       manifest.OSBuildManifest       `json:"manifest"`
		RPMMD          map[string][]rpmmd.PackageSpec `json:"rpmmd"`
		Containers     map[string][]container.Spec    `json:"containers,omitempty"`
		OSTreeCommits  map[string][]ostree.CommitSpec `json:"ostree-commits,omitempty"`
		NoImageInfo    bool                           `json:"no-image-info"`
	}{
		cr, ms, pkgs, containers, commits, true,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data for %q: %s\n", filename, err.Error())
	}
	b = append(b, '\n') // add new line at end of file
	fpath := filepath.Join(path, filename)
	fp, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %s\n", fpath, err.Error())
	}
	defer fp.Close()
	if _, err := fp.Write(b); err != nil {
		return fmt.Errorf("failed to write output file %q: %s\n", fpath, err.Error())
	}
	return nil
}

func filterRepos(repos []repository, typeName string) []repository {
	filtered := make([]repository, 0)
	for _, repo := range repos {
		if len(repo.ImageTypeTags) == 0 {
			filtered = append(filtered, repo)
		} else {
			for _, tt := range repo.ImageTypeTags {
				if tt == typeName {
					filtered = append(filtered, repo)
					break
				}
			}
		}
	}
	return filtered
}

// collects requests from a formatRequestMap based on image type
func requestsByImageType(requestMap formatRequestMap) map[string]map[string]manifestRequest {
	imgTypeRequestMap := make(map[string]map[string]manifestRequest)

	for name, req := range requestMap {
		it := req.ComposeRequest.ImageType
		reqs := imgTypeRequestMap[it]
		if reqs == nil {
			reqs = make(map[string]manifestRequest)
		}
		reqs[name] = req
		imgTypeRequestMap[it] = reqs
	}
	return imgTypeRequestMap
}

func archIsSupported(req manifestRequest, arch string) bool {
	if len(req.SupportedArches) == 0 {
		// none specified: all arches supported implicitly
		return true
	}
	for _, supportedArch := range req.SupportedArches {
		if supportedArch == arch {
			return true
		}
	}
	return false
}

func mergeOverrides(base, overrides composeRequest) composeRequest {
	// NOTE: in most cases overrides are only used for blueprints and probably
	// doesn't make sense to use them for most fields, but let's merge all
	// regardless
	merged := composeRequest(base)
	if overrides.Blueprint != nil {
		merged.Blueprint = overrides.Blueprint
	}

	if overrides.Filename != "" {
		merged.Filename = overrides.Filename
	}
	if overrides.ImageType != "" {
		merged.ImageType = overrides.ImageType
	}
	if overrides.OSTree != nil {
		merged.OSTree = overrides.OSTree
	}
	if overrides.Distro != "" {
		merged.Distro = overrides.Distro
	}
	if overrides.Arch != "" {
		merged.Arch = overrides.Arch
	}
	if len(overrides.Repositories) > 0 {
		merged.Repositories = overrides.Repositories
	}
	return merged
}

func main() {
	// common args
	var outputDir, cacheRoot string
	var nWorkers uint
	flag.StringVar(&outputDir, "output", "test/data/manifests/", "manifest store directory")
	flag.UintVar(&nWorkers, "workers", 16, "number of workers to run concurrently")
	flag.StringVar(&cacheRoot, "cache", "/tmp/rpmmd", "rpm metadata cache directory")

	// manifest selection args
	var arches, distros, imgTypes multiValue
	flag.Var(&arches, "arches", "comma-separated list of architectures")
	flag.Var(&distros, "distros", "comma-separated list of distributions")
	flag.Var(&imgTypes, "images", "comma-separated list of image types")

	flag.Parse()

	// nWorkers cannot be larger than uint32
	if nWorkers > math.MaxUint32 {
		panic(fmt.Sprintf("--workers must be %d or less.", math.MaxUint32))
	}

	seedArg := int64(0)
	darm := readRepos()
	distroFac := distrofactory.NewDefault()
	jobs := make([]manifestJob, 0)

	requestMap := loadFormatRequestMap()
	itRequestMap := requestsByImageType(requestMap)

	if err := os.MkdirAll(outputDir, 0770); err != nil {
		panic(fmt.Sprintf("failed to create target directory: %s", err.Error()))
	}

	fmt.Println("Collecting jobs")
	if len(distros) == 0 {
		distros = darm.ListDistros()
	}
	for _, distroName := range distros {
		distribution := distroFac.GetDistro(distroName)
		if distribution == nil {
			fmt.Fprintf(os.Stderr, "invalid distro name %q\n", distroName)
			continue
		}

		distroArches := arches
		if len(distroArches) == 0 {
			distroArches = distribution.ListArches()
		}
		for _, archName := range distroArches {
			arch, err := distribution.GetArch(archName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid arch name %q for distro %q: %s\n", archName, distroName, err.Error())
				continue
			}

			daImgTypes := imgTypes
			if len(daImgTypes) == 0 {
				daImgTypes = arch.ListImageTypes()
			}
			for _, imgTypeName := range daImgTypes {
				imgType, err := arch.GetImageType(imgTypeName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "invalid image type %q for distro %q and arch %q: %s\n", imgTypeName, distroName, archName, err.Error())
					continue
				}

				// get repositories
				repos := darm[distroName][archName]
				if len(repos) == 0 {
					fmt.Printf("no repositories defined for %s/%s\n", distroName, archName)
					fmt.Println("Skipping")
					continue
				}

				// run through jobs from request map that match the image type
				for jobName, req := range itRequestMap[imgTypeName] {
					// skip if architecture is not supported
					if !archIsSupported(req, archName) {
						continue
					}

					// check for distro-specific overrides
					if or, exist := req.Overrides[distroName]; exist {
						req.ComposeRequest = mergeOverrides(req.ComposeRequest, or)
					}

					composeReq := req.ComposeRequest
					composeReq.Repositories = filterRepos(repos, imgTypeName)

					job := makeManifestJob(jobName, imgType, composeReq, distribution, archName, seedArg, outputDir, cacheRoot)
					jobs = append(jobs, job)
				}
			}
		}
	}

	nJobs := len(jobs)
	fmt.Printf("Collected %d jobs\n", nJobs)
	// nWorkers has been tested to be <= math.MaxUint32
	/* #nosec G115 */
	wq := newWorkerQueue(uint32(nWorkers), uint32(nJobs))
	wq.start()
	fmt.Printf("Initialised %d workers\n", nWorkers)
	fmt.Printf("Submitting %d jobs... ", nJobs)
	for _, j := range jobs {
		wq.submitJob(j)
	}
	fmt.Println("done")
	errs := wq.wait()
	exit := 0
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Encountered %d errors:\n", len(errs))
		for idx, err := range errs {
			fmt.Fprintf(os.Stderr, "%3d: %s\n", idx, err.Error())
		}
		exit = 1
	}
	fmt.Printf("RPM metadata cache kept in %s\n", cacheRoot)
	os.Exit(exit)
}
