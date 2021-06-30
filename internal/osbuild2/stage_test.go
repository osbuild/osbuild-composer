package osbuild2

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
			name: "chrony",
			fields: fields{
				Type:    "org.osbuild.chrony",
				Options: &ChronyStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.chrony","options":{"timeservers":null}}`),
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
				Type:    "org.osbuild.dracut.conf",
				Options: &DracutConfStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.dracut.conf","options":{}}`),
			},
		},
		{
			name: "dracut.conf-data",
			fields: fields{
				Type: "org.osbuild.dracut.conf",
				Options: &DracutConfStageOptions{
					ConfigFiles: map[string]DracutConfigFile{
						"sgdisk.conf": {
							Install: []string{"sgdisk"},
						},
						"testing.conf": {
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
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.dracut.conf","options":{"configuration_files":{"sgdisk.conf":{"install_items":["sgdisk"]},"testing.conf":{"compress":"xz","add_dracutmodules":["floppy"],"omit_dracutmodules":["nouveau"],"add_drivers":["driver1"],"force_drivers":["driver2"],"filesystems":["ext4"],"install_items":["file1"],"early_microcode":false,"reproducible":false}}}}`),
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
			name: "modprobe",
			fields: fields{
				Type:    "org.osbuild.modprobe",
				Options: &ModprobeStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.modprobe","options":{}}`),
			},
		},
		{
			name: "modprobe-data",
			fields: fields{
				Type: "org.osbuild.modprobe",
				Options: &ModprobeStageOptions{
					ConfigFiles: map[string]ModprobeConfigCmdList{
						"disallow-modules.conf": {
							&ModprobeConfigCmdBlacklist{
								Command:    "blacklist",
								Modulename: "nouveau",
							},
							&ModprobeConfigCmdBlacklist{
								Command:    "blacklist",
								Modulename: "floppy",
							},
						},
						"disallow-additional-modules.conf": {
							&ModprobeConfigCmdBlacklist{
								Command:    "blacklist",
								Modulename: "my-module",
							},
						},
					},
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.modprobe","options":{"configuration_files":{"disallow-additional-modules.conf":[{"command":"blacklist","modulename":"my-module"}],"disallow-modules.conf":[{"command":"blacklist","modulename":"nouveau"},{"command":"blacklist","modulename":"floppy"}]}}}`),
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
				},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.rhsm","options":{"dnf-plugins":{"product-id":{"enabled":false},"subscription-manager":{"enabled":false}}}}`),
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
			name: "users",
			fields: fields{
				Type:    "org.osbuild.users",
				Options: &UsersStageOptions{},
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.users","options":{"users":null}}`),
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
