package osbuild

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
)

const (
	kickstartStageType               = "org.osbuild.kickstart"
	KickstartPathInteractiveDefaults = "/usr/share/anaconda/interactive-defaults.ks"
	KickstartPathOSBuild             = "/osbuild.ks"
)

type KickstartStageOptions struct {
	// Where to place the kickstart file
	Path string `json:"path"`

	OSTreeCommit    *OSTreeCommitOptions    `json:"ostree,omitempty"`
	OSTreeContainer *OSTreeContainerOptions `json:"ostreecontainer,omitempty"`

	LiveIMG *LiveIMGOptions `json:"liveimg,omitempty"`

	Users map[string]UsersStageOptionsUser `json:"users,omitempty"`

	Groups map[string]GroupsStageOptionsGroup `json:"groups,omitempty"`

	Lang         string               `json:"lang,omitempty"`
	Keyboard     string               `json:"keyboard,omitempty"`
	Timezone     string               `json:"timezone,omitempty"`
	DisplayMode  string               `json:"display_mode,omitempty"`
	Reboot       *RebootOptions       `json:"reboot,omitempty"`
	RootPassword *RootPasswordOptions `json:"rootpw,omitempty"`
	ZeroMBR      bool                 `json:"zerombr,omitempty"`
	ClearPart    *ClearPartOptions    `json:"clearpart,omitempty"`
	AutoPart     *AutoPartOptions     `json:"autopart,omitempty"`
	Network      []NetworkOptions     `json:"network,omitempty"`
	Bootloader   *BootloaderOptions   `json:"bootloader,omitempty"`
	Post         []PostOptions        `json:"%post,omitempty"`
}

type BootloaderOptions struct {
	Append string `json:"append"`
}

type LiveIMGOptions struct {
	URL string `json:"url"`
}

type OSTreeCommitOptions struct {
	OSName string `json:"osname"`
	Remote string `json:"remote"`
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	GPG    bool   `json:"gpg"`
}

type OSTreeContainerOptions struct {
	StateRoot             string `json:"stateroot"`
	URL                   string `json:"url"`
	Transport             string `json:"transport"`
	Remote                string `json:"remote"`
	SignatureVerification bool   `json:"signatureverification"`
}

type RebootOptions struct {
	Eject bool `json:"eject,omitempty"`
	KExec bool `json:"kexec,omitempty"`
}

type ClearPartOptions struct {
	All       bool     `json:"all,omitempty"`
	InitLabel bool     `json:"initlabel,omitempty"`
	Drives    []string `json:"drives,omitempty"`
	List      []string `json:"list,omitempty"`
	Linux     bool     `json:"linux,omitempty"`
}

type AutoPartOptions struct {
	Type             string `json:"type,omitempty"`
	FSType           string `json:"fstype,omitempty"`
	NoLVM            bool   `json:"nolvm,omitempty"`
	Encrypted        bool   `json:"encrypted,omitempty"`
	PassPhrase       string `json:"passphrase,omitempty"`
	EscrowCert       string `json:"escrowcert,omitempty"`
	BackupPassPhrase bool   `json:"backuppassphrase,omitempty"`
	Cipher           string `json:"cipher,omitempty"`
	LuksVersion      string `json:"luks-version,omitempty"`
	PBKdf            string `json:"pbkdf,omitempty"`
	PBKdfMemory      int    `json:"pbkdf-memory,omitempty"`
	PBKdfTime        int    `json:"pbkdf-time,omitempty"`
	PBKdfIterations  int    `json:"pbkdf-iterations,omitempty"`
	NoHome           bool   `json:"nohome,omitempty"`
}

type NetworkOptions struct {
	Activate    *bool    `json:"activate,omitempty"`
	BootProto   string   `json:"bootproto,omitempty"`
	Device      string   `json:"device,omitempty"`
	OnBoot      string   `json:"onboot,omitempty"`
	IP          string   `json:"ip,omitempty"`
	IPV6        string   `json:"ipv6,omitempty"`
	Gateway     string   `json:"gateway,omitempty"`
	IPV6Gateway string   `json:"ipv6gateway,omitempty"`
	Nameservers []string `json:"nameservers,omitempty"`
	Netmask     string   `json:"netmask,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ESSid       string   `json:"essid,omitempty"`
	WPAKey      string   `json:"wpakey,omitempty"`
}

type RootPasswordOptions struct {
	Lock      bool   `json:"lock,omitempty"`
	PlainText bool   `json:"plaintext,omitempty"`
	IsCrypted bool   `json:"iscrypted,omitempty"`
	AllowSSH  bool   `json:"allow_ssh,omitempty"`
	Password  string `json:"password,omitempty"`
}

type PostOptions struct {
	ErrorOnFail bool     `json:"erroronfail,omitempty"`
	Interpreter string   `json:"interpreter,omitempty"`
	Log         string   `json:"log,omitempty"`
	NoChroot    bool     `json:"nochroot,omitempty"`
	Commands    []string `json:"commands"`
}

func (KickstartStageOptions) isStageOptions() {}

// Creates an Anaconda kickstart file
func NewKickstartStage(options *KickstartStageOptions) *Stage {
	return &Stage{
		Type:    kickstartStageType,
		Options: options,
	}
}

// adjustRootUserOptions handles any options relating to the root user for the
// kickstart stage. It returns the [RootPasswordOptions] if necessary and
// modifies the userCustomizations array accordingly. It also validates that no
// unsupported options are set.
//
// User options for the root user have no effect in kickstart. In other
// words, the
//
//	user --name "root" ...
//
// line is ignored, because when the users are being processed, the root
// user already exists. To set a root password, the rootpw command must be
// used. If an SSH key is added for the root user however, we need to keep
// the root user account in the kickstart options, because osbuild will use
// it to add the sshkey line for the root user. Unfortunately, this means
// that we will get a bare user line for root that will have no effect.
// See also https://github.com/osbuild/osbuild/issues/2178
func adjustRootUserOptions(userOptions map[string]UsersStageOptionsUser) (*RootPasswordOptions, error) {
	var rootpw *RootPasswordOptions
	for name, user := range userOptions {
		if name == "root" {
			if user.Password != nil {
				rootpw = &RootPasswordOptions{
					IsCrypted: true, // NewUserStageOptions() always encrypts plaintext passwords
					Password:  *user.Password,
				}

				// remove the password since the --password option for the user
				// kickstart command has no effect on root
				user.Password = nil
			}

			// return an error if any other field is set (except SSH)
			unsupportedOptionsSet := make([]string, 0, 7)
			if user.ExpireDate != nil {
				unsupportedOptionsSet = append(unsupportedOptionsSet, "expiredate")
			}
			if user.ForcePasswordReset != nil {
				unsupportedOptionsSet = append(unsupportedOptionsSet, "force_password_reset")
			}
			if user.GID != nil {
				unsupportedOptionsSet = append(unsupportedOptionsSet, "gid")
			}
			if user.Groups != nil {
				unsupportedOptionsSet = append(unsupportedOptionsSet, "groups")
			}
			if user.Home != nil {
				unsupportedOptionsSet = append(unsupportedOptionsSet, "home")
			}
			if user.Shell != nil {
				unsupportedOptionsSet = append(unsupportedOptionsSet, "shell")
			}
			if user.UID != nil {
				unsupportedOptionsSet = append(unsupportedOptionsSet, "uid")
			}
			if len(unsupportedOptionsSet) > 0 {
				return nil, fmt.Errorf("unsupported options for user \"root\": %s", strings.Join(unsupportedOptionsSet, ", "))
			}

			// if the ssh key is set, update the options in the map (unset
			// password), otherwise remove it entirely
			if user.Key != nil {
				userOptions[name] = user
			} else {
				delete(userOptions, name)
			}
			return rootpw, nil
		}
	}
	return nil, nil
}

func NewKickstartStageOptions(
	path string,
	userCustomizations []users.User,
	groupCustomizations []users.Group) (*KickstartStageOptions, error) {

	invalidPathRegex := regexp.MustCompile(invalidPathRegex)
	if invalidPathRegex.FindAllString(path, -1) != nil {
		return nil, fmt.Errorf("%s: kickstart path %q is invalid", kickstartStageType, path)
	}

	var users map[string]UsersStageOptionsUser
	if usersOptions, err := NewUsersStageOptions(userCustomizations, false); err != nil {
		return nil, err
	} else if usersOptions != nil {
		users = usersOptions.Users
	}

	var groups map[string]GroupsStageOptionsGroup
	if groupsOptions := NewGroupsStageOptions(groupCustomizations); groupsOptions != nil {
		groups = groupsOptions.Groups
	}

	rootpw, err := adjustRootUserOptions(users)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", kickstartStageType, err)
	}

	return &KickstartStageOptions{
		Path:         path,
		OSTreeCommit: nil,
		LiveIMG:      nil,
		Users:        users,
		Groups:       groups,
		RootPassword: rootpw,
	}, nil
}

func NewKickstartStageOptionsWithOSTreeCommit(
	path string,
	userCustomizations []users.User,
	groupCustomizations []users.Group,
	ostreeURL string,
	ostreeRef string,
	ostreeRemote string,
	osName string) (*KickstartStageOptions, error) {

	options, err := NewKickstartStageOptions(path, userCustomizations, groupCustomizations)

	if err != nil {
		return nil, err
	}

	if ostreeURL != "" {
		ostreeCommitOptions := &OSTreeCommitOptions{
			OSName: osName,
			Remote: ostreeRemote,
			URL:    ostreeURL,
			Ref:    ostreeRef,
			GPG:    false,
		}

		options.OSTreeCommit = ostreeCommitOptions
	}

	return options, nil
}

func NewKickstartStageOptionsWithOSTreeContainer(
	path string,
	userCustomizations []users.User,
	groupCustomizations []users.Group,
	containerURL string,
	containerTransport string,
	containerRemote string,
	containerStateRoot string) (*KickstartStageOptions, error) {

	options, err := NewKickstartStageOptions(path, userCustomizations, groupCustomizations)

	if err != nil {
		return nil, err
	}

	if containerURL != "" {
		ostreeContainerOptions := &OSTreeContainerOptions{
			StateRoot:             containerStateRoot,
			URL:                   containerURL,
			Remote:                containerRemote,
			Transport:             containerTransport,
			SignatureVerification: false,
		}

		options.OSTreeContainer = ostreeContainerOptions
	}

	return options, nil
}

func NewKickstartStageOptionsWithLiveIMG(
	path string,
	userCustomizations []users.User,
	groupCustomizations []users.Group,
	imageURL string) (*KickstartStageOptions, error) {

	options, err := NewKickstartStageOptions(path, userCustomizations, groupCustomizations)

	if err != nil {
		return nil, err
	}

	if imageURL != "" {
		liveImg := &LiveIMGOptions{
			URL: imageURL,
		}
		options.LiveIMG = liveImg
	}

	return options, nil
}

// IncludeRaw is used for adding raw text as an extension to the kickstart
// file. First it changes the filename of the existing kickstart stage options
// and then creates a new file with the given raw content and an %include
// statement at the top that points to the renamed file. The new raw content is
// generated in place of the original file and is returned as an fsnode.File.
// The raw content *should not* contain the %include statement.
func (options *KickstartStageOptions) IncludeRaw(raw string) (*fsnode.File, error) {
	// TODO: flip the way this function includes one kickstart in another when
	// we add an include option to the kickstart stage so that the raw part
	// remains intact, our own kickstart file remains the "primary", and we
	// %include the raw kickstart from our own.
	origPath := options.Path
	origName := filepath.Base(origPath)

	ext := filepath.Ext(origName)

	// file.ext -> file-base.ext
	newBaseName := strings.TrimSuffix(origName, ext) + "-base" + ext
	options.Path = filepath.Join("/", newBaseName)

	// include must point to full path when booted
	includePath := filepath.Join("/run/install/repo", newBaseName)

	rawBits := fmt.Sprintf("%%include %s\n%s", includePath, raw)
	return fsnode.NewFile(origPath, nil, nil, nil, []byte(rawBits))
}
