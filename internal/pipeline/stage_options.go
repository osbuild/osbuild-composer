package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// selinuxStageOptions returns the options for the org.osbuild.selinux stage.
// Setting the argument to 'true' relabels the '/usr/bin/cp'
// binariy with 'install_exec_t'. This should be set in the build root.
func selinuxStageOptions(labelcp bool) *osbuild2.SELinuxStageOptions {
	options := &osbuild2.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
	if labelcp {
		options.Labels = map[string]string{
			"/usr/bin/cp": "system_u:object_r:install_exec_t:s0",
		}
	}
	return options
}
