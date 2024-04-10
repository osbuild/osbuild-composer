package oscap

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/customizations/fsnode"
)

type Profile string

func (p Profile) String() string {
	return string(p)
}

const (
	AnssiBp28Enhanced     Profile = "xccdf_org.ssgproject.content_profile_anssi_bp28_enhanced"
	AnssiBp28High         Profile = "xccdf_org.ssgproject.content_profile_anssi_bp28_high"
	AnssiBp28Intermediary Profile = "xccdf_org.ssgproject.content_profile_anssi_bp28_intermediary"
	AnssiBp28Minimal      Profile = "xccdf_org.ssgproject.content_profile_anssi_bp28_minimal"
	CcnAdvanced           Profile = "xccdf_org.ssgproject.content_profile_ccn_advanced"
	CcnBasic              Profile = "xccdf_org.ssgproject.content_profile_ccn_basic"
	CcnIntermediate       Profile = "xccdf_org.ssgproject.content_profile_ccn_intermediate"
	Cis                   Profile = "xccdf_org.ssgproject.content_profile_cis"
	CisServerL1           Profile = "xccdf_org.ssgproject.content_profile_cis_server_l1"
	CisWorkstationL1      Profile = "xccdf_org.ssgproject.content_profile_cis_workstation_l1"
	CisWorkstationL2      Profile = "xccdf_org.ssgproject.content_profile_cis_workstation_l2"
	Cui                   Profile = "xccdf_org.ssgproject.content_profile_cui"
	E8                    Profile = "xccdf_org.ssgproject.content_profile_e8"
	Hippa                 Profile = "xccdf_org.ssgproject.content_profile_hipaa"
	IsmO                  Profile = "xccdf_org.ssgproject.content_profile_ism_o"
	Ospp                  Profile = "xccdf_org.ssgproject.content_profile_ospp"
	PciDss                Profile = "xccdf_org.ssgproject.content_profile_pci-dss"
	Standard              Profile = "xccdf_org.ssgproject.content_profile_standard"
	Stig                  Profile = "xccdf_org.ssgproject.content_profile_stig"
	StigGui               Profile = "xccdf_org.ssgproject.content_profile_stig_gui"

	// datastream fallbacks
	defaultFedoraDatastream  string = "/usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml"
	defaultCentos8Datastream string = "/usr/share/xml/scap/ssg/content/ssg-centos8-ds.xml"
	defaultCentos9Datastream string = "/usr/share/xml/scap/ssg/content/ssg-cs9-ds.xml"
	defaultRHEL8Datastream   string = "/usr/share/xml/scap/ssg/content/ssg-rhel8-ds.xml"
	defaultRHEL9Datastream   string = "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml"

	// tailoring directory path
	tailoringDirPath string = "/usr/share/xml/osbuild-openscap-data"
)

func DefaultFedoraDatastream() string {
	return defaultFedoraDatastream
}

func DefaultRHEL8Datastream(isRHEL bool) string {
	if isRHEL {
		return defaultRHEL8Datastream
	}
	return defaultCentos8Datastream
}

func DefaultRHEL9Datastream(isRHEL bool) string {
	if isRHEL {
		return defaultRHEL9Datastream
	}
	return defaultCentos9Datastream
}

func IsProfileAllowed(profile string, allowlist []Profile) bool {
	for _, a := range allowlist {
		if a.String() == profile {
			return true
		}
		// this enables a user to specify
		// the full profile or the short
		// profile id
		if strings.HasSuffix(a.String(), profile) {
			return true
		}
	}

	return false
}

func GetTailoringFile(profile string) (string, string, *fsnode.Directory, error) {
	newProfile := fmt.Sprintf("%s_osbuild_tailoring", profile)
	path := filepath.Join(tailoringDirPath, "tailoring.xml")

	tailoringDir, err := fsnode.NewDirectory(tailoringDirPath, nil, nil, nil, true)
	if err != nil {
		return "", "", nil, err
	}

	return newProfile, path, tailoringDir, nil
}
