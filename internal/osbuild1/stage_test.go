package osbuild1

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestStage_UnmarshalJSON(t *testing.T) {
	nullUUID := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	type fields struct {
		Name    string
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
				data: []byte(`{"name":"org.osbuild.foo","options":{"bar":null}`),
			},
			wantErr: true,
		},
		{
			name: "unknown stage",
			args: args{
				data: []byte(`{"name":"org.osbuild.foo","options":{"bar":null}}`),
			},
			wantErr: true,
		},
		{
			name: "missing options",
			args: args{
				data: []byte(`{"name":"org.osbuild.locale"}`),
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
			name: "chrony",
			fields: fields{
				Name:    "org.osbuild.chrony",
				Options: &ChronyStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.chrony","options":{"timeservers":null}}`),
			},
		},
		{
			name: "firewall",
			fields: fields{
				Name:    "org.osbuild.firewall",
				Options: &FirewallStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.firewall","options":{}}`),
			},
		},
		{
			name: "fix-bls",
			fields: fields{
				Name:    "org.osbuild.fix-bls",
				Options: &FixBLSStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.fix-bls","options":{}}`),
			},
		},
		{
			name: "fstab",
			fields: fields{
				Name:    "org.osbuild.fstab",
				Options: &FSTabStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.fstab","options":{"filesystems":null}}`),
			},
		},
		{
			name: "groups",
			fields: fields{
				Name:    "org.osbuild.groups",
				Options: &GroupsStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.groups","options":{"groups":null}}`),
			},
		},
		{
			name: "grub2",
			fields: fields{
				Name: "org.osbuild.grub2",
				Options: &GRUB2StageOptions{
					RootFilesystemUUID: nullUUID,
				},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.grub2","options":{"root_fs_uuid":"00000000-0000-0000-0000-000000000000"}}`),
			},
		},
		{
			name: "grub2-uefi",
			fields: fields{
				Name: "org.osbuild.grub2",
				Options: &GRUB2StageOptions{
					RootFilesystemUUID: nullUUID,
					UEFI: &GRUB2UEFI{
						Vendor: "vendor",
					},
				},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.grub2","options":{"root_fs_uuid":"00000000-0000-0000-0000-000000000000","uefi":{"vendor":"vendor"}}}`),
			},
		},
		{
			name: "grub2-separate-boot",
			fields: fields{
				Name: "org.osbuild.grub2",
				Options: &GRUB2StageOptions{
					RootFilesystemUUID: nullUUID,
					BootFilesystemUUID: &nullUUID,
				},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.grub2","options":{"root_fs_uuid":"00000000-0000-0000-0000-000000000000","boot_fs_uuid":"00000000-0000-0000-0000-000000000000"}}`),
			},
		},
		{
			name: "hostname",
			fields: fields{
				Name:    "org.osbuild.hostname",
				Options: &HostnameStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.hostname","options":{"hostname":""}}`),
			},
		},
		{
			name: "keymap",
			fields: fields{
				Name:    "org.osbuild.keymap",
				Options: &KeymapStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.keymap","options":{"keymap":""}}`),
			},
		},
		{
			name: "locale",
			fields: fields{
				Name:    "org.osbuild.locale",
				Options: &LocaleStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.locale","options":{"language":""}}`),
			},
		},
		{
			name: "rhsm-empty",
			fields: fields{
				Name:    "org.osbuild.rhsm",
				Options: &RHSMStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.rhsm","options":{}}`),
			},
		},
		{
			name: "rhsm",
			fields: fields{
				Name: "org.osbuild.rhsm",
				Options: &RHSMStageOptions{
					DnfPlugins: &RHSMStageOptionsDnfPlugins{
						ProductID: &RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
						SubscriptionManager: &RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
					},
				},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.rhsm","options":{"dnf-plugins":{"product-id":{"enabled":false},"subscription-manager":{"enabled":false}}}}`),
			},
		},
		{
			name: "rpm-empty",
			fields: fields{
				Name:    "org.osbuild.rpm",
				Options: &RPMStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.rpm","options":{"packages":null}}`),
			},
		},
		{
			name: "rpm",
			fields: fields{
				Name: "org.osbuild.rpm",
				Options: &RPMStageOptions{
					GPGKeys: []string{"key1", "key2"},
					Packages: []RPMPackage{
						{
							Checksum: "checksum1",
						},
						{
							Checksum: "checksum2",
							CheckGPG: true,
						},
					},
				},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.rpm","options":{"gpgkeys":["key1","key2"],"packages":[{"checksum":"checksum1"},{"checksum":"checksum2","check_gpg":true}]}}`),
			},
		},
		{
			name: "rpm-ostree",
			fields: fields{
				Name: "org.osbuild.rpm-ostree",
				Options: &RPMOSTreeStageOptions{
					EtcGroupMembers: []string{
						"wheel",
					},
				},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.rpm-ostree","options":{"etc_group_members":["wheel"]}}`),
			},
		},
		{
			name: "script",
			fields: fields{
				Name:    "org.osbuild.script",
				Options: &ScriptStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.script","options":{"script":""}}`),
			},
		},
		{
			name: "selinux",
			fields: fields{
				Name:    "org.osbuild.selinux",
				Options: &SELinuxStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.selinux","options":{"file_contexts":""}}`),
			},
		},
		{
			name: "systemd",
			fields: fields{
				Name:    "org.osbuild.systemd",
				Options: &SystemdStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.systemd","options":{}}`),
			},
		},
		{
			name: "systemd-enabled",
			fields: fields{
				Name: "org.osbuild.systemd",
				Options: &SystemdStageOptions{
					EnabledServices: []string{"foo.service"},
				},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.systemd","options":{"enabled_services":["foo.service"]}}`),
			},
		},
		{
			name: "timezone",
			fields: fields{
				Name:    "org.osbuild.timezone",
				Options: &TimezoneStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.timezone","options":{"zone":""}}`),
			},
		},
		{
			name: "users",
			fields: fields{
				Name:    "org.osbuild.users",
				Options: &UsersStageOptions{},
			},
			args: args{
				data: []byte(`{"name":"org.osbuild.users","options":{"users":null}}`),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := &Stage{
				Name:    tt.fields.Name,
				Options: tt.fields.Options,
			}
			var gotStage Stage
			if err := gotStage.UnmarshalJSON(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Stage.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			gotBytes, err := json.Marshal(stage)
			if err != nil {
				t.Errorf("Could not marshal stage: %v", err)
			}
			if !bytes.Equal(gotBytes, tt.args.data) {
				t.Errorf("Expected `%v`, got `%v`", string(tt.args.data), string(gotBytes))
			}
			if !reflect.DeepEqual(&gotStage, stage) {
				t.Errorf("got {%v, %v}, expected {%v, %v}", gotStage.Name, gotStage.Options, stage.Name, stage.Options)
			}
		})
	}
}
