package osbuild2

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/common"
)

func TestStage_UnmarshalJSON(t *testing.T) {
	nullUUID := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	type fields struct {
		Type    string
		Options StageOptions
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "invalid json",
			args: args{
				data: []byte(`{"type":"org.osbuild.foo","options":{"bar":null}}`),
			},
			wantErr: true,
		},
		{
			name: "unknown stage",
			args: args{
				data: []byte(`{"type":"org.osbuild.foo","options":{"bar":null}}`),
			},
			wantErr: true,
		},
		{
			name: "missing options",
			args: args{
				data: []byte(`{"type":"org.osbuild.locale"}`),
			},
			wantErr: true,
		},
		{
			name: "missing name",
			args: args{
				data: []byte(`{"foo":null,"options":{"bar":null}}`),
			},
			wantErr: true,
		},
		{
			name: "authselect",
			fields: fields{
				Type: "org.osbuild.authselect",
				Options: &AuthselectStageOptions{
					Profile: "sssd",
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.authselect","options":{"profile":"sssd"}}`),
			},
		},
		{
			name: "authselect-features",
			fields: fields{
				Type: "org.osbuild.authselect",
				Options: &AuthselectStageOptions{
					Profile:  "nis",
					Features: []string{"with-ecryptfs", "with-mkhomedir"},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.authselect","options":{"profile":"nis","features":["with-ecryptfs","with-mkhomedir"]}}`),
			},
		},
		{
			name: "cloud-init",
			fields: fields{
				Type: "org.osbuild.cloud-init",
				Options: &CloudInitStageOptions{
					Filename: "00-default_user.cfg",
					Config: CloudInitConfigFile{
						SystemInfo: &CloudInitConfigSystemInfo{
							DefaultUser: &CloudInitConfigDefaultUser{
								Name: "ec2-user",
							},
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.cloud-init","options":{"filename":"00-default_user.cfg","config":{"system_info":{"default_user":{"name":"ec2-user"}}}}}`),
			},
		},
		{
			name: "chrony-timeservers",
			fields: fields{
				Type: "org.osbuild.chrony",
				Options: &ChronyStageOptions{
					Timeservers: []string{
						"ntp1.example.com",
						"ntp2.example.com",
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.chrony","options":{"timeservers":["ntp1.example.com","ntp2.example.com"]}}`),
			},
		},
		{
			name: "chrony-servers",
			fields: fields{
				Type: "org.osbuild.chrony",
				Options: &ChronyStageOptions{
					Servers: []ChronyConfigServer{
						{
							Hostname: "127.0.0.1",
							Minpoll:  common.IntToPtr(0),
							Maxpoll:  common.IntToPtr(4),
							Iburst:   common.BoolToPtr(true),
							Prefer:   common.BoolToPtr(false),
						},
						{
							Hostname: "ntp.example.com",
						},
					},
					LeapsecTz: common.StringToPtr(""),
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.chrony","options":{"servers":[{"hostname":"127.0.0.1","minpoll":0,"maxpoll":4,"iburst":true,"prefer":false},{"hostname":"ntp.example.com"}],"leapsectz":""}}`),
			},
		},
		{
			name: "dnf-config",
			fields: fields{
				Type:    "org.osbuild.dnf.config",
				Options: &DNFConfigStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.dnf.config","options":{}}`),
			},
		},
		{
			name: "dnf-automatic-config",
			fields: fields{
				Type:    "org.osbuild.dnf-automatic.config",
				Options: &DNFAutomaticConfigStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.dnf-automatic.config","options":{}}`),
			},
		},
		{
			name: "dracut",
			fields: fields{
				Type:    "org.osbuild.dracut",
				Options: &DracutStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.dracut","options":{"kernel":null}}`),
			},
		},
		{
			name: "dracut.conf",
			fields: fields{
				Type: "org.osbuild.dracut.conf",
				Options: &DracutConfStageOptions{
					Filename: "testing.conf",
					Config: DracutConfigFile{
						Compress:       "xz",
						AddModules:     []string{"floppy"},
						OmitModules:    []string{"nouveau"},
						AddDrivers:     []string{"driver1"},
						ForceDrivers:   []string{"driver2"},
						Filesystems:    []string{"ext4"},
						Install:        []string{"file1"},
						EarlyMicrocode: common.BoolToPtr(false),
						Reproducible:   common.BoolToPtr(false),
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.dracut.conf","options":{"filename":"testing.conf","config":{"compress":"xz","add_dracutmodules":["floppy"],"omit_dracutmodules":["nouveau"],"add_drivers":["driver1"],"force_drivers":["driver2"],"filesystems":["ext4"],"install_items":["file1"],"early_microcode":false,"reproducible":false}}}`),
			},
		},
		{
			name: "firewall",
			fields: fields{
				Type:    "org.osbuild.firewall",
				Options: &FirewallStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.firewall","options":{}}`),
			},
		},
		{
			name: "fix-bls",
			fields: fields{
				Type:    "org.osbuild.fix-bls",
				Options: &FixBLSStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.fix-bls","options":{}}`),
			},
		},
		{
			name: "fix-bls-empty-prefix",
			fields: fields{
				Type: "org.osbuild.fix-bls",
				Options: &FixBLSStageOptions{
					Prefix: common.StringToPtr(""),
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.fix-bls","options":{"prefix":""}}`),
			},
		},
		{
			name: "fstab",
			fields: fields{
				Type:    "org.osbuild.fstab",
				Options: &FSTabStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.fstab","options":{"filesystems":null}}`),
			},
		},
		{
			name: "groups",
			fields: fields{
				Type:    "org.osbuild.groups",
				Options: &GroupsStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.groups","options":{"groups":null}}`),
			},
		},
		{
			name: "grub2",
			fields: fields{
				Type: "org.osbuild.grub2",
				Options: &GRUB2StageOptions{
					RootFilesystemUUID: nullUUID,
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.grub2","options":{"root_fs_uuid":"00000000-0000-0000-0000-000000000000"}}`),
			},
		},
		{
			name: "grub2-uefi",
			fields: fields{
				Type: "org.osbuild.grub2",
				Options: &GRUB2StageOptions{
					RootFilesystemUUID: nullUUID,
					UEFI: &GRUB2UEFI{
						Vendor: "vendor",
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.grub2","options":{"root_fs_uuid":"00000000-0000-0000-0000-000000000000","uefi":{"vendor":"vendor"}}}`),
			},
		},
		{
			name: "grub2-separate-boot",
			fields: fields{
				Type: "org.osbuild.grub2",
				Options: &GRUB2StageOptions{
					RootFilesystemUUID: nullUUID,
					BootFilesystemUUID: &nullUUID,
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.grub2","options":{"root_fs_uuid":"00000000-0000-0000-0000-000000000000","boot_fs_uuid":"00000000-0000-0000-0000-000000000000"}}`),
			},
		},
		{
			name: "hostname",
			fields: fields{
				Type:    "org.osbuild.hostname",
				Options: &HostnameStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.hostname","options":{"hostname":""}}`),
			},
		},
		{
			name: "keymap",
			fields: fields{
				Type:    "org.osbuild.keymap",
				Options: &KeymapStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.keymap","options":{"keymap":""}}`),
			},
		},
		{
			name: "keymap-x11-keymap",
			fields: fields{
				Type: "org.osbuild.keymap",
				Options: &KeymapStageOptions{
					Keymap: "us",
					X11Keymap: &X11KeymapOptions{
						Layouts: []string{"cz"},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.keymap","options":{"keymap":"us","x11-keymap":{"layouts":["cz"]}}}`),
			},
		},
		{
			name: "modprobe",
			fields: fields{
				Type: "org.osbuild.modprobe",
				Options: &ModprobeStageOptions{
					Filename: "disallow-modules.conf",
					Commands: ModprobeConfigCmdList{
						&ModprobeConfigCmdBlacklist{
							Command:    "blacklist",
							Modulename: "nouveau",
						},
						&ModprobeConfigCmdBlacklist{
							Command:    "blacklist",
							Modulename: "floppy",
						},
						&ModprobeConfigCmdInstall{
							Command:    "install",
							Modulename: "nf_conntrack",
							Cmdline:    "/usr/sbin/modprobe --ignore-install nf_conntrack $CMDLINE_OPTS && /usr/sbin/sysctl --quiet --pattern 'net[.]netfilter[.]nf_conntrack.*' --system",
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.modprobe","options":{"filename":"disallow-modules.conf","commands":[{"command":"blacklist","modulename":"nouveau"},{"command":"blacklist","modulename":"floppy"},{"command":"install","modulename":"nf_conntrack","cmdline":"/usr/sbin/modprobe --ignore-install nf_conntrack $CMDLINE_OPTS \u0026\u0026 /usr/sbin/sysctl --quiet --pattern 'net[.]netfilter[.]nf_conntrack.*' --system"}]}}`),
			},
		},
		{
			name: "locale",
			fields: fields{
				Type:    "org.osbuild.locale",
				Options: &LocaleStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.locale","options":{"language":""}}`),
			},
		},
		{
			name: "pam-limits-conf-str",
			fields: fields{
				Type: "org.osbuild.pam.limits.conf",
				Options: &PamLimitsConfStageOptions{
					Filename: "example.conf",
					Config: []PamLimitsConfigLine{
						{
							Domain: "user1",
							Type:   PamLimitsTypeHard,
							Item:   PamLimitsItemNofile,
							Value:  PamLimitsValueUnlimited,
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.pam.limits.conf","options":{"filename":"example.conf","config":[{"domain":"user1","type":"hard","item":"nofile","value":"unlimited"}]}}`),
			},
		},
		{
			name: "pam-limits-conf-int",
			fields: fields{
				Type: "org.osbuild.pam.limits.conf",
				Options: &PamLimitsConfStageOptions{
					Filename: "example.conf",
					Config: []PamLimitsConfigLine{
						{
							Domain: "user1",
							Type:   PamLimitsTypeHard,
							Item:   PamLimitsItemNofile,
							Value:  PamLimitsValueInt(-1),
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.pam.limits.conf","options":{"filename":"example.conf","config":[{"domain":"user1","type":"hard","item":"nofile","value":-1}]}}`),
			},
		},
		{
			name: "tuned",
			fields: fields{
				Type: "org.osbuild.tuned",
				Options: &TunedStageOptions{
					Profiles: []string{"sap-hana"},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.tuned","options":{"profiles":["sap-hana"]}}`),
			},
		},
		{
			name: "rhsm-empty",
			fields: fields{
				Type:    "org.osbuild.rhsm",
				Options: &RHSMStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.rhsm","options":{}}`),
			},
		},
		{
			name: "rhsm",
			fields: fields{
				Type: "org.osbuild.rhsm",
				Options: &RHSMStageOptions{
					DnfPlugins: &RHSMStageOptionsDnfPlugins{
						ProductID: &RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
						SubscriptionManager: &RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
					},
					SubMan: &RHSMStageOptionsSubMan{
						Rhsm: &SubManConfigRHSMSection{
							ManageRepos: common.BoolToPtr(false),
						},
						Rhsmcertd: &SubManConfigRHSMCERTDSection{
							AutoRegistration: common.BoolToPtr(true),
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.rhsm","options":{"dnf-plugins":{"product-id":{"enabled":false},"subscription-manager":{"enabled":false}},"subscription-manager":{"rhsm":{"manage_repos":false},"rhsmcertd":{"auto_registration":true}}}}`),
			},
		},
		{
			name: "script",
			fields: fields{
				Type:    "org.osbuild.script",
				Options: &ScriptStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.script","options":{"script":""}}`),
			},
		},
		{
			name: "selinux",
			fields: fields{
				Type:    "org.osbuild.selinux",
				Options: &SELinuxStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.selinux","options":{"file_contexts":""}}`),
			},
		},
		{
			name: "selinux-force_autorelabel",
			fields: fields{
				Type: "org.osbuild.selinux",
				Options: &SELinuxStageOptions{
					ForceAutorelabel: common.BoolToPtr(true),
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.selinux","options":{"file_contexts":"","force_autorelabel":true}}`),
			},
		},
		{
			name: "selinux.config-empty",
			fields: fields{
				Type:    "org.osbuild.selinux.config",
				Options: &SELinuxConfigStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.selinux.config","options":{}}`),
			},
		},
		{
			name: "selinux.config",
			fields: fields{
				Type: "org.osbuild.selinux.config",
				Options: &SELinuxConfigStageOptions{
					State: SELinuxStatePermissive,
					Type:  SELinuxTypeMinimum,
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.selinux.config","options":{"state":"permissive","type":"minimum"}}`),
			},
		},
		{
			name: "sysconfig",
			fields: fields{
				Type:    "org.osbuild.sysconfig",
				Options: &SysconfigStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.sysconfig","options":{}}`),
			},
		},
		{
			name: "sysconfig-data",
			fields: fields{
				Type: "org.osbuild.sysconfig",
				Options: &SysconfigStageOptions{
					Kernel: &SysconfigKernelOptions{
						UpdateDefault: true,
						DefaultKernel: "kernel",
					},
					Network: &SysconfigNetworkOptions{
						Networking: true,
						NoZeroConf: true,
					},
					NetworkScripts: &NetworkScriptsOptions{
						IfcfgFiles: map[string]IfcfgFile{
							"eth0": {
								Device:    "eth0",
								Bootproto: IfcfgBootprotoDHCP,
								OnBoot:    common.BoolToPtr(true),
								Type:      IfcfgTypeEthernet,
								UserCtl:   common.BoolToPtr(true),
								PeerDNS:   common.BoolToPtr(true),
								IPv6Init:  common.BoolToPtr(false),
							},
							"eth1": {
								Device:    "eth1",
								Bootproto: IfcfgBootprotoDHCP,
								OnBoot:    common.BoolToPtr(true),
								Type:      IfcfgTypeEthernet,
								UserCtl:   common.BoolToPtr(false),
								PeerDNS:   common.BoolToPtr(true),
								IPv6Init:  common.BoolToPtr(true),
							},
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.sysconfig","options":{"kernel":{"update_default":true,"default_kernel":"kernel"},"network":{"networking":true,"no_zero_conf":true},"network-scripts":{"ifcfg":{"eth0":{"bootproto":"dhcp","device":"eth0","ipv6init":false,"onboot":true,"peerdns":true,"type":"Ethernet","userctl":true},"eth1":{"bootproto":"dhcp","device":"eth1","ipv6init":true,"onboot":true,"peerdns":true,"type":"Ethernet","userctl":false}}}}}`),
			},
		},
		{
			name: "sysctld",
			fields: fields{
				Type: "org.osbuild.sysctld",
				Options: &SysctldStageOptions{
					Filename: "example.conf",
					Config: []SysctldConfigLine{
						{
							Key:   "net.ipv4.conf.*.rp_filter",
							Value: "2",
						},
						{
							Key: "-net.ipv4.conf.all.rp_filter",
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.sysctld","options":{"filename":"example.conf","config":[{"key":"net.ipv4.conf.*.rp_filter","value":"2"},{"key":"-net.ipv4.conf.all.rp_filter"}]}}`),
			},
		},
		{
			name: "systemd",
			fields: fields{
				Type:    "org.osbuild.systemd",
				Options: &SystemdStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.systemd","options":{}}`),
			},
		},
		{
			name: "systemd-enabled",
			fields: fields{
				Type: "org.osbuild.systemd",
				Options: &SystemdStageOptions{
					EnabledServices: []string{"foo.service"},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.systemd","options":{"enabled_services":["foo.service"]}}`),
			},
		},
		{
			name: "systemd-unit-dropins",
			fields: fields{
				Type: "org.osbuild.systemd.unit",
				Options: &SystemdUnitStageOptions{
					Unit:   "nm-cloud-setup.service",
					Dropin: "10-rh-enable-for-ec2.conf",
					Config: SystemdServiceUnitDropin{
						Service: &SystemdUnitServiceSection{
							Environment: "NM_CLOUD_SETUP_EC2=yes",
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.systemd.unit","options":{"unit":"nm-cloud-setup.service","dropin":"10-rh-enable-for-ec2.conf","config":{"Service":{"Environment":"NM_CLOUD_SETUP_EC2=yes"}}}}`),
			},
		},
		{
			name: "systemd-logind",
			fields: fields{
				Type: "org.osbuild.systemd-logind",
				Options: &SystemdLogindStageOptions{
					Filename: "10-ec2-getty-fix.conf",
					Config: SystemdLogindConfigDropin{
						Login: SystemdLogindConfigLoginSection{
							NAutoVTs: common.IntToPtr(0),
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.systemd-logind","options":{"filename":"10-ec2-getty-fix.conf","config":{"Login":{"NAutoVTs":0}}}}`),
			},
		},
		{
			name: "timezone",
			fields: fields{
				Type:    "org.osbuild.timezone",
				Options: &TimezoneStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.timezone","options":{"zone":""}}`),
			},
		},
		{
			name: "tmpfilesd",
			fields: fields{
				Type: "org.osbuild.tmpfilesd",
				Options: &TmpfilesdStageOptions{
					Filename: "example.conf",
					Config: []TmpfilesdConfigLine{
						{
							Type: "d",
							Path: "/tmp/my-example-path",
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.tmpfilesd","options":{"filename":"example.conf","config":[{"type":"d","path":"/tmp/my-example-path"}]}}`),
			},
		},
		{
			name: "users",
			fields: fields{
				Type:    "org.osbuild.users",
				Options: &UsersStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.users","options":{"users":null}}`),
			},
		},
		{
			name: "sshd.config",
			fields: fields{
				Type:    "org.osbuild.sshd.config",
				Options: &SshdConfigStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.sshd.config","options":{"config":{}}}`),
			},
		},
		{
			name: "sshd.config-data1",
			fields: fields{
				Type: "org.osbuild.sshd.config",
				Options: &SshdConfigStageOptions{
					Config: SshdConfigConfig{
						PasswordAuthentication:          common.BoolToPtr(false),
						ChallengeResponseAuthentication: common.BoolToPtr(false),
						ClientAliveInterval:             common.IntToPtr(42),
						PermitRootLogin:                 PermitRootLoginValueNo,
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.sshd.config","options":{"config":{"PasswordAuthentication":false,"ChallengeResponseAuthentication":false,"ClientAliveInterval":42,"PermitRootLogin":false}}}`),
			},
		},
		{
			name: "sshd.config-data2",
			fields: fields{
				Type: "org.osbuild.sshd.config",
				Options: &SshdConfigStageOptions{
					Config: SshdConfigConfig{
						PasswordAuthentication:          common.BoolToPtr(false),
						ChallengeResponseAuthentication: common.BoolToPtr(false),
						ClientAliveInterval:             common.IntToPtr(42),
						PermitRootLogin:                 PermitRootLoginValueForcedCommandsOnly,
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.sshd.config","options":{"config":{"PasswordAuthentication":false,"ChallengeResponseAuthentication":false,"ClientAliveInterval":42,"PermitRootLogin":"forced-commands-only"}}}`),
			},
		},
		{
			name: "authconfig",
			fields: fields{
				Type:    "org.osbuild.authconfig",
				Options: &AuthconfigStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.authconfig","options":{}}`),
			},
		},
		{
			name: "pwquality.conf",
			fields: fields{
				Type:    "org.osbuild.pwquality.conf",
				Options: &PwqualityConfStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.pwquality.conf","options":{"config":{}}}`),
			},
		},
		{
			name: "yum.config",
			fields: fields{
				Type:    "org.osbuild.yum.config",
				Options: &YumConfigStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.yum.config","options":{}}`),
			},
		},
		{
			name: "yum.repos",
			fields: fields{
				Type: "org.osbuild.yum.repos",
				Options: &YumReposStageOptions{
					Filename: "test.repo",
					Repos: []YumRepository{
						{
							Id:      "my-repo",
							BaseURL: []string{"http://example.org/repo"},
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.yum.repos","options":{"filename":"test.repo","repos":[{"id":"my-repo","baseurl":["http://example.org/repo"]}]}}`),
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := &Stage{
				Type:    tt.fields.Type,
				Options: tt.fields.Options,
			}
			var gotStage Stage
			if err := gotStage.UnmarshalJSON(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Stage.UnmarshalJSON() error = %v, wantErr %v [idx: %d]", err, tt.wantErr, idx)
			}
			if tt.wantErr {
				return
			}
			gotBytes, err := json.Marshal(stage)
			if err != nil {
				t.Errorf("Could not marshal stage: %v", err)
			}
			if !bytes.Equal(gotBytes, tt.args.data) {
				t.Errorf("Expected `%v`, got `%v` [idx: %d]", string(tt.args.data), string(gotBytes), idx)
			}
			if !reflect.DeepEqual(&gotStage, stage) {
				t.Errorf("got {%v, %v}, expected {%v, %v} [%d]", gotStage.Type, gotStage.Options, stage.Type, stage.Options, idx)
			}
		})
	}
}

// Test new stages that have Inputs (osbuild v2 schema)
func TestStageV2_UnmarshalJSON(t *testing.T) {
	type fields struct {
		Type    string
		Options StageOptions
		Inputs  Inputs
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "rpm-empty",
			fields: fields{
				Type:    "org.osbuild.rpm",
				Options: &RPMStageOptions{},
				Inputs:  &RPMStageInputs{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.rpm","inputs":{"packages":null},"options":{}}`),
			},
		},
		{
			name: "rpm",
			fields: fields{
				Type: "org.osbuild.rpm",
				Inputs: &RPMStageInputs{
					Packages: &RPMStageInput{
						References: RPMStageReferences{
							"checksum1",
							"checksum2",
						},
					},
				},
				Options: &RPMStageOptions{
					GPGKeys: []string{"key1", "key2"},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.rpm","inputs":{"packages":{"type":"","origin":"","references":["checksum1","checksum2"]}},"options":{"gpgkeys":["key1","key2"]}}`),
			},
		},
		{
			name: "ostree-preptree",
			fields: fields{
				Type: "org.osbuild.ostree.preptree",
				Options: &OSTreePrepTreeStageOptions{
					EtcGroupMembers: []string{
						"wheel",
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.ostree.preptree","options":{"etc_group_members":["wheel"]}}`),
			},
		},
		{
			name: "xz",
			fields: fields{
				Type: "org.osbuild.xz",
				Options: &XzStageOptions{
					Filename: "image.raw.xz",
				},
				Inputs: NewFilesInputs(NewFilesInputReferencesPipeline("os", "image.raw")),
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.xz","inputs":{"file":{"type":"org.osbuild.files","origin":"org.osbuild.pipeline","references":{"name:os":{"file":"image.raw"}}}},"options":{"filename":"image.raw.xz"}}`),
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := &Stage{
				Type:    tt.fields.Type,
				Options: tt.fields.Options,
				Inputs:  tt.fields.Inputs,
			}
			var gotStage Stage
			if err := gotStage.UnmarshalJSON(tt.args.data); (err != nil) != tt.wantErr {
				println("data: ", string(tt.args.data))
				t.Errorf("Stage.UnmarshalJSON() error = %v, wantErr %v [idx: %d]", err, tt.wantErr, idx)
			}
			if tt.wantErr {
				return
			}
			gotBytes, err := json.Marshal(stage)
			if err != nil {
				t.Errorf("Could not marshal stage: %v", err)
			}
			if !bytes.Equal(gotBytes, tt.args.data) {
				t.Errorf("Expected `%v`, got `%v` [idx: %d]", string(tt.args.data), string(gotBytes), idx)
			}
			if !reflect.DeepEqual(&gotStage, stage) {
				t.Errorf("got {%v, %v, %v}, expected {%v, %v, %v} [%d]", gotStage.Type, gotStage.Options, gotStage.Inputs, stage.Type, stage.Options, stage.Inputs, idx)
			}
		})
	}
}
