package vmware

import (
	"os"
	"os/exec"
	"strings"
)

func OpenAsStreamOptimizedVmdk(imagePath string) (*os.File, error) {
	newPath := strings.TrimSuffix(imagePath, ".vmdk") + "-stream.vmdk"
	cmd := exec.Command(
		"/usr/bin/qemu-img", "convert", "-O", "vmdk", "-o", "subformat=streamOptimized",
		imagePath, newPath)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(newPath)
	if err != nil {
		return nil, err
	}
	return f, err
}
