package vmware

import (
	"os"
	"os/exec"
)

func OpenAsStreamOptimizedVmdk(imagePath string) (*os.File, error) {
	newPath := imagePath + ".stream"
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
	err = os.Remove(newPath)
	if err != nil {
		return nil, err
	}
	return f, err
}
