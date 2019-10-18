package blueprint

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func Test_qcow2Output_translate(t *testing.T) {
	type args struct {
		b *Blueprint
	}
	var emptyPipeline pipeline.Pipeline
	json.Unmarshal([]byte(`{"build":{"stages":[{"name":"org.osbuild.dnf","options":{"repos":[{"metalink":"https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever\u0026arch=$basearch","gpgkey":"F1D8 EC98 F241 AAF2 0DF6  9420 EF3C 111F CFC6 59B9","checksum":"sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97"}],"packages":["dnf","e2fsprogs","policycoreutils","qemu-img","systemd","grub2-pc","tar"],"releasever":"30","basearch":"x86_64"}}]},"stages":[{"name":"org.osbuild.dnf","options":{"repos":[{"metalink":"https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever\u0026arch=$basearch","gpgkey":"F1D8 EC98 F241 AAF2 0DF6  9420 EF3C 111F CFC6 59B9","checksum":"sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97"}],"packages":["@Core","chrony","kernel","selinux-policy-targeted","grub2-pc","spice-vdagent","qemu-guest-agent","xen-libs","langpacks-en"],"releasever":"30","basearch":"x86_64"}},{"name":"org.osbuild.fix-bls","options":{}},{"name":"org.osbuild.locale","options":{"language":"en_US"}},{"name":"org.osbuild.fstab","options":{"filesystems":[{"uuid":"76a22bf4-f153-4541-b6c7-0332c0dfaeac","vfs_type":"extf4","path":"/","options":"defaults","freq":1,"passno":1}]}},{"name":"org.osbuild.grub2","options":{"root_fs_uuid":"76a22bf4-f153-4541-b6c7-0332c0dfaeac","boot_fs_uuid":"00000000-0000-0000-0000-000000000000","kernel_opts":"ro biosdevname=0 net.ifnames=0"}},{"name":"org.osbuild.selinux","options":{"file_contexts":"etc/selinux/targeted/contexts/files/file_contexts"}}],"assembler":{"name":"org.osbuild.qemu","options":{"format":"qcow2","filename":"image.qcow2","ptuuid":"0x14fc63d2","root_fs_uuid":"76a22bf4-f153-4541-b6c7-0332c0dfaeac","size":3221225472}}}`),
		&emptyPipeline)
	tests := []struct {
		name string
		t    *qcow2Output
		args args
		want *pipeline.Pipeline
	}{
		{
			name: "empty-blueprint",
			t:    &qcow2Output{},
			args: args{&Blueprint{}},
			want: &emptyPipeline,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.translate(tt.args.b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("qcow2Output.translate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_qcow2Output_getName(t *testing.T) {
	tests := []struct {
		name string
		t    *qcow2Output
		want string
	}{
		{
			t:    &qcow2Output{},
			want: "image.qcow2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getName(); got != tt.want {
				t.Errorf("qcow2Output.getName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_qcow2Output_getMime(t *testing.T) {
	tests := []struct {
		name string
		t    *qcow2Output
		want string
	}{
		{
			t:    &qcow2Output{},
			want: "application/x-qemu-disk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getMime(); got != tt.want {
				t.Errorf("qcow2Output.getMime() = %v, want %v", got, tt.want)
			}
		})
	}
}
