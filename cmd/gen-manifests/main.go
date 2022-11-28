// Standalone executable for generating all test manifests in parallel.
// Collects list of image types from the distro list.  Must be run from the
// root of the repository and reads tools/test-case-generators/repos.json for
// repositories tools/test-case-generators/format-request-map.json for
// customizations Collects errors and failures and prints them after all jobs
// are finished.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
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
	BaseURL        string   `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKey         string   `json:"gpgkey,omitempty"`
	CheckGPG       bool     `json:"check_gpg,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
}

type ostreeOptions struct {
	Ref    string `json:"ref"`
	URL    string `json:"url"`
	Parent string `json:"parent"`
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
		options.OSTree = distro.OSTreeImageOptions{
			URL:           cr.OSTree.URL,
			ImageRef:      cr.OSTree.Ref,
			FetchChecksum: cr.OSTree.Parent,
		}
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
		}

		containerSpecs, err := resolveContainers(bp.Containers, archName)
		if err != nil {
			return fmt.Errorf("[%s] container resolution failed: %s", filename, err.Error())
		}

		if options.OSTree.ImageRef == "" {
			// use default OSTreeRef for image type
			options.OSTree.ImageRef = imgType.OSTreeRef()
		}

		packageSpecs, err := depsolve(cacheDir, imgType, bp, options, repos, distribution, archName)
		if err != nil {
			err = fmt.Errorf("[%s] depsolve failed: %s", filename, err.Error())
			return
		}
		if packageSpecs == nil {
			err = fmt.Errorf("[%s] nil package specs", filename)
			return
		}
		manifest, err := imgType.Manifest(cr.Blueprint.Customizations, options, repos, packageSpecs, containerSpecs, seedArg)
		if err != nil {
			err = fmt.Errorf("[%s] failed: %s", filename, err)
			return
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
		err = save(manifest, packageSpecs, request, path, filename)
		return
	}
	return job
}

type DistroArchRepoMap map[string]map[string][]repository

func convertRepo(r repository) rpmmd.RepoConfig {
	return rpmmd.RepoConfig{
		Name:           r.Name,
		BaseURL:        r.BaseURL,
		Metalink:       r.Metalink,
		MirrorList:     r.MirrorList,
		GPGKey:         r.GPGKey,
		CheckGPG:       r.CheckGPG,
		MetadataExpire: r.MetadataExpire,
		ImageTypeTags:  r.ImageTypeTags,
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

func resolveContainers(containers []blueprint.Container, archName string) ([]container.Spec, error) {
	resolver := container.NewResolver(archName)

	for _, c := range containers {
		resolver.Add(c.Source, c.Name, c.TLSVerify)
	}

	return resolver.Finish()
}

func depsolve(cacheDir string, imageType distro.ImageType, bp blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, d distro.Distro, arch string) (map[string][]rpmmd.PackageSpec, error) {
	solver := dnfjson.NewSolver(d.ModulePlatformID(), d.Releasever(), arch, cacheDir)
	solver.SetDNFJSONPath("./dnf-json")
	packageSets := imageType.PackageSets(bp, options, repos)
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	for name, pkgSet := range packageSets {
		res, err := solver.Depsolve(pkgSet)
		if err != nil {
			return nil, err
		}
		depsolvedSets[name] = res
	}
	return depsolvedSets, nil
}

func save(manifest distro.Manifest, pkgs map[string][]rpmmd.PackageSpec, cr composeRequest, path, filename string) error {
	data := struct {
		ComposeRequest composeRequest                 `json:"compose-request"`
		Manifest       distro.Manifest                `json:"manifest"`
		RPMMD          map[string][]rpmmd.PackageSpec `json:"rpmmd"`
		NoImageInfo    bool                           `json:"no-image-info"`
	}{
		cr, manifest, pkgs, true,
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
	var nWorkers int
	flag.StringVar(&outputDir, "output", "test/data/manifests.plain/", "manifest store directory")
	flag.IntVar(&nWorkers, "workers", 16, "number of workers to run concurrently")
	flag.StringVar(&cacheRoot, "cache", "/tmp/rpmmd", "rpm metadata cache directory")

	// manifest selection args
	var arches, distros, imgTypes multiValue
	flag.Var(&arches, "arches", "comma-separated list of architectures")
	flag.Var(&distros, "distros", "comma-separated list of distributions")
	flag.Var(&imgTypes, "images", "comma-separated list of image types")

	flag.Parse()

	seedArg := int64(0)
	darm := readRepos()
	distroReg := distroregistry.NewDefault()
	jobs := make([]manifestJob, 0)

	requestMap := loadFormatRequestMap()
	itRequestMap := requestsByImageType(requestMap)

	if err := os.MkdirAll(outputDir, 0770); err != nil {
		panic(fmt.Sprintf("failed to create target directory: %s", err.Error()))
	}

	fmt.Println("Collecting jobs")
	if len(distros) == 0 {
		distros = distroReg.List()
	}
	for _, distroName := range distros {
		distribution := distroReg.GetDistro(distroName)
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
