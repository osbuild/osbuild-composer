package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/osbuild"
)

type Subscription struct {
	Base

	Subscription *subscription.ImageOptions

	// Custom directories and files to create in the pipeline
	Directories []*fsnode.Directory
	Files       []*fsnode.File
}

// NewSubscription creates a new subscription pipeline for creating files
// required to register a system on first boot.
// The pipeline is intended to be used to create the files necessary for
// registering a system, but outside the OS tree, so they can be copied to
// other locations in the tree after they're created (for example, to an ISO).
func NewSubscription(buildPipeline Build, subOptions *subscription.ImageOptions) *Subscription {
	name := "subscription"
	p := &Subscription{
		Base:         NewBase(name, buildPipeline),
		Subscription: subOptions,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *Subscription) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()
	if p.Subscription != nil {
		serviceDir, err := fsnode.NewDirectory("/etc/systemd/system", nil, nil, nil, true)
		if err != nil {
			panic(err)
		}
		p.Directories = append(p.Directories, serviceDir)

		subStage, subDirs, subFiles, _, err := subscriptionService(*p.Subscription, &subscriptionServiceOptions{InsightsOnBoot: true, UnitPath: osbuild.EtcUnitPath})
		if err != nil {
			panic(err)
		}
		p.Directories = append(p.Directories, subDirs...)
		p.Files = append(p.Files, subFiles...)

		pipeline.AddStages(osbuild.GenDirectoryNodesStages(p.Directories)...)
		pipeline.AddStages(osbuild.GenFileNodesStages(p.Files)...)
		pipeline.AddStage(subStage)
	}
	return pipeline
}

func (p *Subscription) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}

type subscriptionServiceOptions struct {
	// InsightsOnBoot controls whether the insights client service will be
	// modified (with a drop-in) to run on boot as well as on a timer.
	InsightsOnBoot bool

	// UnitPath controls the path where the systemd unit will be created,
	// /usr/lib/systemd or /etc/systemd.
	UnitPath osbuild.SystemdUnitPath

	// Indicates if rhc should be set to permissive when creating the registration script
	PermissiveRHC bool
}

// subscriptionService creates the necessary stage and modifications to the
// pipeline for activating a system on first boot.
//
// If subscription settings are included there are 3 possible setups:
// - Register the system with rhc and enable Insights
// - Register with subscription-manager, no Insights or rhc
// - Register with subscription-manager and enable Insights, no rhc
func subscriptionService(
	subscriptionOptions subscription.ImageOptions,
	serviceOptions *subscriptionServiceOptions) (*osbuild.Stage, []*fsnode.Directory, []*fsnode.File, []string, error) {
	dirs := make([]*fsnode.Directory, 0)
	files := make([]*fsnode.File, 0)
	services := make([]string, 0)

	insightsOnBoot := false
	unitPath := osbuild.UsrUnitPath
	permissiveRHC := false
	if serviceOptions != nil {
		insightsOnBoot = serviceOptions.InsightsOnBoot
		if serviceOptions.UnitPath != "" {
			unitPath = serviceOptions.UnitPath
		}
		permissiveRHC = serviceOptions.PermissiveRHC
	}

	// Write a key file that will contain the org ID and activation key to be sourced in the systemd service.
	// The file will also act as the ConditionFirstBoot file.
	subkeyFilepath := "/etc/osbuild-subscription-register.env"
	subkeyContent := fmt.Sprintf("ORG_ID=%s\nACTIVATION_KEY=%s", subscriptionOptions.Organization, subscriptionOptions.ActivationKey)

	// NOTE: Ownership is left as nil:nil, which implicitly creates files as
	// root:root. Adding an explicit owner requires chroot to run the
	// org.osbuild.chown stage, which we can't run in the subscription pipeline
	// since it has no packages.
	if subkeyFile, err := fsnode.NewFile(subkeyFilepath, nil, nil, nil, []byte(subkeyContent)); err == nil {
		files = append(files, subkeyFile)
	} else {
		return nil, nil, nil, nil, err
	}

	var commands []string
	if subscriptionOptions.Rhc {
		var curlToAssociateSystem string
		// Use rhc for registration instead of subscription manager
		rhcConnect := fmt.Sprintf("/usr/bin/rhc connect --organization=${ORG_ID} --activation-key=${ACTIVATION_KEY} --server %s", subscriptionOptions.ServerUrl)
		if subscriptionOptions.TemplateUUID != "" {
			curlToAssociateSystem = getCurlToAssociateSystem(subscriptionOptions)
		} else if subscriptionOptions.TemplateName != "" {
			rhcConnect += fmt.Sprintf(" --content-template=\"%s\"", subscriptionOptions.TemplateName)
		}
		commands = append(commands, rhcConnect)
		if permissiveRHC {
			// execute the rhc post install script as the selinuxenabled check doesn't work in the buildroot container
			commands = append(commands, "/usr/sbin/semanage permissive --add rhcd_t")
		}
		// register to template if template uuid is specified
		if curlToAssociateSystem != "" {
			commands = append(commands, curlToAssociateSystem)
			commands = append(commands, "/usr/sbin/subscription-manager refresh")
		}
		if insightsOnBoot {
			icDir, icFile, err := runInsightsClientOnBoot()
			if err != nil {
				return nil, nil, nil, nil, err
			}
			dirs = append(dirs, icDir)
			files = append(files, icFile)
		}
	} else {
		commands = []string{fmt.Sprintf("/usr/sbin/subscription-manager register --org=${ORG_ID} --activationkey=${ACTIVATION_KEY} --serverurl %s --baseurl %s", subscriptionOptions.ServerUrl, subscriptionOptions.BaseUrl)}

		// Insights is optional when using subscription-manager
		if subscriptionOptions.Insights {
			commands = append(commands, "/usr/bin/insights-client --register")
			// register to template if template is specified
			if subscriptionOptions.TemplateUUID != "" {
				curlToAssociateSystem := getCurlToAssociateSystem(subscriptionOptions)
				commands = append(commands, curlToAssociateSystem)
				commands = append(commands, "/usr/sbin/subscription-manager refresh")
			}
			if insightsOnBoot {
				icDir, icFile, err := runInsightsClientOnBoot()
				if err != nil {
					return nil, nil, nil, nil, err
				}
				dirs = append(dirs, icDir)
				files = append(files, icFile)
			}
		}
	}

	commands = append(commands, fmt.Sprintf("/usr/bin/rm %s", subkeyFilepath))

	subscribeServiceFile := "osbuild-subscription-register.service"
	regServiceStageOptions := &osbuild.SystemdUnitCreateStageOptions{
		Filename: subscribeServiceFile,
		UnitType: "system",
		UnitPath: unitPath,
		Config: osbuild.SystemdUnit{
			Unit: &osbuild.UnitSection{
				Description:         "First-boot service for registering with Red Hat subscription manager and/or insights",
				ConditionPathExists: []string{subkeyFilepath},
				Wants:               []string{"network-online.target"},
				After:               []string{"network-online.target"},
			},
			Service: &osbuild.ServiceSection{
				Type:            osbuild.OneshotServiceType,
				RemainAfterExit: false,
				ExecStart:       commands,
				EnvironmentFile: []string{subkeyFilepath},
			},
			Install: &osbuild.InstallSection{
				WantedBy: []string{"default.target"},
			},
		},
	}
	services = append(services, subscribeServiceFile)
	unitStage := osbuild.NewSystemdUnitCreateStage(regServiceStageOptions)
	return unitStage, dirs, files, services, nil
}

// Creates a drop-in file for the insights-client service to run on boot and
// enables the service. This is only meant for ostree-based systems.
func runInsightsClientOnBoot() (*fsnode.Directory, *fsnode.File, error) {
	// Insights-client collection must occur at boot time  so
	// that the current ostree commit hash can be reflected
	// after upgrade. Otherwise, the upgrade shows as failed in
	// the console UI.
	// Add a drop-in file that enables insights-client.service to
	// run on successful boot.
	// See https://issues.redhat.com/browse/HMS-4031
	//
	// NOTE(akoutsou): drop-in files can normally be created with the
	// org.osbuild.systemd.unit stage but the stage doesn't support
	// all the options we need. This is a temporary workaround
	// until we get the stage updated to support everything we need.
	icDropinFilepath, icDropinContents := insightsClientDropin()

	// NOTE: Ownership is left as nil:nil, which implicitly creates files as
	// root:root. Adding an explicit owner requires chroot to run the
	// org.osbuild.chown stage, which we can't run in the subscription pipeline
	// since it has no packages.
	icDropinDirectory, err := fsnode.NewDirectory(filepath.Dir(icDropinFilepath), nil, nil, nil, true)
	if err != nil {
		return nil, nil, err
	}
	icDropinFile, err := fsnode.NewFile(icDropinFilepath, nil, nil, nil, []byte(icDropinContents))
	if err != nil {
		return nil, nil, err
	}
	return icDropinDirectory, icDropinFile, nil
}

// Filename and contents for the insights-client-boot service drop-in.
// This is a temporary workaround until the org.osbuild.systemd.unit stage
// gains support for all the options we need.
//
// The insights-client-boot.service is defined as first boot with a conditional file
// that gets removed when it runs (similar to the registration service).
// We are overriding that here to run at every boot to collect changes after upgrades, etc.
func insightsClientDropin() (string, string) {
	return "/etc/systemd/system/insights-client-boot.service.d/override.conf", `[Unit]
Requisite=greenboot-healthcheck.service
After=network-online.target greenboot-healthcheck.service osbuild-first-boot.service osbuild-subscription-register.service
ConditionPathExists=
[Service]
ExecStartPre=
[Install]
WantedBy=multi-user.target
`
}

func getCurlToAssociateSystem(subscriptionOptions subscription.ImageOptions) string {
	patchURL := strings.TrimSuffix(subscriptionOptions.PatchURL, "/")
	addTemplateURL := fmt.Sprintf("%s/templates/%s/subscribed-systems", patchURL, subscriptionOptions.TemplateUUID)
	curlToAssociateSystem := fmt.Sprintf("curl -v --retry 5 --cert /etc/pki/consumer/cert.pem --key /etc/pki/consumer/key.pem -X PATCH %s", addTemplateURL)
	if subscriptionOptions.Proxy != "" {
		curlToAssociateSystem += fmt.Sprintf(" --proxy %s", subscriptionOptions.Proxy)
	}
	return curlToAssociateSystem
}
