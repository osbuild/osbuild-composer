package fedora30

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"

	"github.com/google/uuid"
)

var f30GPGKey string = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBFturGcBEACv0xBo91V2n0uEC2vh69ywCiSyvUgN/AQH8EZpCVtM7NyjKgKm
bbY4G3R0M3ir1xXmvUDvK0493/qOiFrjkplvzXFTGpPTi0ypqGgxc5d0ohRA1M75
L+0AIlXoOgHQ358/c4uO8X0JAA1NYxCkAW1KSJgFJ3RjukrfqSHWthS1d4o8fhHy
KJKEnirE5hHqB50dafXrBfgZdaOs3C6ppRIePFe2o4vUEapMTCHFw0woQR8Ah4/R
n7Z9G9Ln+0Cinmy0nbIDiZJ+pgLAXCOWBfDUzcOjDGKvcpoZharA07c0q1/5ojzO
4F0Fh4g/BUmtrASwHfcIbjHyCSr1j/3Iz883iy07gJY5Yhiuaqmp0o0f9fgHkG53
2xCU1owmACqaIBNQMukvXRDtB2GJMuKa/asTZDP6R5re+iXs7+s9ohcRRAKGyAyc
YKIQKcaA+6M8T7/G+TPHZX6HJWqJJiYB+EC2ERblpvq9TPlLguEWcmvjbVc31nyq
SDoO3ncFWKFmVsbQPTbP+pKUmlLfJwtb5XqxNR5GEXSwVv4I7IqBmJz1MmRafnBZ
g0FJUtH668GnldO20XbnSVBr820F5SISMXVwCXDXEvGwwiB8Lt8PvqzXnGIFDAu3
DlQI5sxSqpPVWSyw08ppKT2Tpmy8adiBotLfaCFl2VTHwOae48X2dMPBvQARAQAB
tDFGZWRvcmEgKDMwKSA8ZmVkb3JhLTMwLXByaW1hcnlAZmVkb3JhcHJvamVjdC5v
cmc+iQI4BBMBAgAiBQJbbqxnAhsPBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAK
CRDvPBEfz8ZZudTnD/9170LL3nyTVUCFmBjT9wZ4gYnpwtKVPa/pKnxbbS+Bmmac
g9TrT9pZbqOHrNJLiZ3Zx1Hp+8uxr3Lo6kbYwImLhkOEDrf4aP17HfQ6VYFbQZI8
f79OFxWJ7si9+3gfzeh9UYFEqOQfzIjLWFyfnas0OnV/P+RMQ1Zr+vPRqO7AR2va
N9wg+Xl7157dhXPCGYnGMNSoxCbpRs0JNlzvJMuAea5nTTznRaJZtK/xKsqLn51D
K07k9MHVFXakOH8QtMCUglbwfTfIpO5YRq5imxlWbqsYWVQy1WGJFyW6hWC0+RcJ
Ox5zGtOfi4/dN+xJ+ibnbyvy/il7Qm+vyFhCYqIPyS5m2UVJUuao3eApE38k78/o
8aQOTnFQZ+U1Sw+6woFTxjqRQBXlQm2+7Bt3bqGATg4sXXWPbmwdL87Ic+mxn/ml
SMfQux/5k6iAu1kQhwkO2YJn9eII6HIPkW+2m5N1JsUyJQe4cbtZE5Yh3TRA0dm7
+zoBRfCXkOW4krchbgww/ptVmzMMP7GINJdROrJnsGl5FVeid9qHzV7aZycWSma7
CxBYB1J8HCbty5NjtD6XMYRrMLxXugvX6Q4NPPH+2NKjzX4SIDejS6JjgrP3KA3O
pMuo7ZHMfveBngv8yP+ZD/1sS6l+dfExvdaJdOdgFCnp4p3gPbw5+Lv70HrMjA==
=BfZ/
-----END PGP PUBLIC KEY BLOCK-----
`

func getF30Repository() *pipeline.DNFRepository {
	repo := pipeline.NewDNFRepository("https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever&arch=$basearch", "", "")
	repo.SetChecksum("sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97")
	repo.SetGPGKey(f30GPGKey)
	return repo
}

func getF30BuildPipeline() *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "30",
		BaseArchitecture: "x86_64",
	}
	options.AddRepository(getF30Repository())
	options.AddPackage("dnf")
	options.AddPackage("e2fsprogs")
	options.AddPackage("policycoreutils")
	options.AddPackage("qemu-img")
	options.AddPackage("systemd")
	options.AddPackage("grub2-pc")
	options.AddPackage("tar")
	p.AddStage(pipeline.NewDNFStage(options))
	return p
}

func newF30Pipeline(packages []string, excludedPackages []string, blueprint *blueprint.Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{
		BuildPipeline: getF30BuildPipeline(),
	}
	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "30",
		BaseArchitecture: "x86_64",
	}
	options.AddRepository(getF30Repository())
	for _, pkg := range packages {
		options.AddPackage(pkg)
	}
	for _, pkg := range excludedPackages {
		options.ExcludePackage(pkg)
	}

	for _, pkg := range blueprint.Packages {
		options.AddPackage(pkg.ToNameVersion())
	}

	for _, pkg := range blueprint.Modules {
		options.AddPackage(pkg.ToNameVersion())
	}

	for _, group := range blueprint.Groups {
		options.AddPackage(group.Name)
	}

	p.AddStage(pipeline.NewDNFStage(options))

	/* grub2 mangles the BLS snippets, we must fix them manually */
	p.AddStage(pipeline.NewFixBLSStage())

	return p
}

func setBootloader(p *pipeline.Pipeline, kernelOptions string, blueprint *blueprint.Blueprint) {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}

	kernelCustomization := blueprint.GetKernelCustomization()
	if kernelCustomization != nil {
		kernelOptions += " " + kernelCustomization.Append
	}

	p.AddStage(pipeline.NewGRUB2Stage(
		&pipeline.GRUB2StageOptions{
			RootFilesystemUUID: id,
			KernelOptions:      kernelOptions,
		},
	))
}

func setFilesystems(p *pipeline.Pipeline) {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	options := &pipeline.FSTabStageOptions{}
	options.AddFilesystem(id, "ext4", "/", "defaults", 1, 1)
	p.AddStage(pipeline.NewFSTabStage(options))
}

func setFirewall(p *pipeline.Pipeline, enabledServices []string, disabledServices []string, b *blueprint.Blueprint) {
	f := b.GetFirewallCustomization()
	ports := []string{}

	if f != nil {
		if f.Services != nil {
			enabledServices = append(enabledServices, f.Services.Enabled...)
			disabledServices = append(disabledServices, f.Services.Disabled...)
		}
		ports = f.Ports
	}

	if len(enabledServices) == 0 && len(disabledServices) == 0 && len(ports) == 0 {
		return
	}

	p.AddStage(
		pipeline.NewFirewallStage(&pipeline.FirewallStageOptions{
			Ports:            ports,
			EnabledServices:  enabledServices,
			DisabledServices: disabledServices,
		}),
	)
}

func setServices(p *pipeline.Pipeline, enabledServices []string, disabledServices []string, b *blueprint.Blueprint) {
	s := b.GetServicesCustomization()

	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		disabledServices = append(disabledServices, s.Enabled...)
	}

	if len(enabledServices) == 0 && len(disabledServices) == 0 {
		return
	}

	p.AddStage(
		pipeline.NewSystemdStage(&pipeline.SystemdStageOptions{
			EnabledServices:  enabledServices,
			DisabledServices: disabledServices,
		}),
	)
}

func setQemuAssembler(p *pipeline.Pipeline, format string, filename string) {
	p.AddStage(pipeline.NewSELinuxStage(
		&pipeline.SELinuxStageOptions{
			FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
		}))
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	p.Assembler = pipeline.NewQEMUAssembler(
		&pipeline.QEMUAssemblerOptions{
			Format:             format,
			Filename:           filename,
			PTUUID:             "0x14fc63d2",
			RootFilesystemUUDI: id,
			// Azure requires this size to be a multiple of 1MB. If you change this, make sure
			// the size still fulfills this requirement to prevent regressions.
			Size: 3222274048,
		})
}

func setTarAssembler(p *pipeline.Pipeline, filename, compression string) {
	p.AddStage(pipeline.NewSELinuxStage(
		&pipeline.SELinuxStageOptions{
			FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
		}))
	p.Assembler = pipeline.NewTarAssembler(
		&pipeline.TarAssemblerOptions{
			Filename: filename,
		})
}

func setRawFSAssembler(p *pipeline.Pipeline, filename string) {
	p.AddStage(pipeline.NewSELinuxStage(
		&pipeline.SELinuxStageOptions{
			FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
		}))
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	p.Assembler = pipeline.NewRawFSAssembler(
		&pipeline.RawFSAssemblerOptions{
			Filename:           filename,
			RootFilesystemUUDI: id,
			Size:               3222274048,
		})
}
