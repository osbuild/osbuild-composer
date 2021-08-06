package rhel90

import (
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func bootISOMonoStageInputs() *osbuild.BootISOMonoStageInputs {
	rootfsInput := new(osbuild.BootISOMonoStageInput)
	rootfsInput.Type = "org.osbuild.tree"
	rootfsInput.Origin = "org.osbuild.pipeline"
	rootfsInput.References = osbuild.BootISOMonoStageReferences{"name:anaconda-tree"}
	return &osbuild.BootISOMonoStageInputs{
		RootFS: rootfsInput,
	}
}

func rpmStageInputs(specs []rpmmd.PackageSpec) *osbuild.RPMStageInputs {
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

func ostreePullStageInputs(origin, source, commitRef string) *osbuild.OSTreePullStageInputs {
	pullStageInput := new(osbuild.OSTreePullStageInput)
	pullStageInput.Type = "org.osbuild.ostree"
	pullStageInput.Origin = origin

	inputRefs := make(map[string]osbuild.OSTreePullStageReference)
	inputRefs[source] = osbuild.OSTreePullStageReference{Ref: commitRef}
	pullStageInput.References = inputRefs
	return &osbuild.OSTreePullStageInputs{Commits: pullStageInput}
}

func xorrisofsStageInputs() *osbuild.XorrisofsStageInputs {
	input := new(osbuild.XorrisofsStageInput)
	input.Type = "org.osbuild.tree"
	input.Origin = "org.osbuild.pipeline"
	input.References = osbuild.XorrisofsStageReferences{"name:bootiso-tree"}
	return &osbuild.XorrisofsStageInputs{Tree: input}
}

func copyPipelineTreeInputs(name, inputPipeline string) *osbuild.CopyStageInputs {
	inputName := "root-tree"
	treeInput := osbuild.CopyStageInput{}
	treeInput.Type = "org.osbuild.tree"
	treeInput.Origin = "org.osbuild.pipeline"
	treeInput.References = []string{"name:" + inputPipeline}
	return &osbuild.CopyStageInputs{inputName: treeInput}
}

func qemuStageInputs(stage, file string) *osbuild.QEMUStageInputs {
	stageKey := "name:" + stage
	ref := map[string]osbuild.QEMUFile{
		stageKey: {
			File: file,
		},
	}
	input := new(osbuild.QEMUStageInput)
	input.Type = "org.osbuild.files"
	input.Origin = "org.osbuild.pipeline"
	input.References = ref
	return &osbuild.QEMUStageInputs{Image: input}
}
