package vmware

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vmware/govmomi/govc/cli"
	_ "github.com/vmware/govmomi/govc/importx"
)

type Credentials struct {
	Host       string
	Username   string
	Password   string
	Datacenter string
	Cluster    string
	Datastore  string
}

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

func UploadImage(creds Credentials, imagePath, imageName string) error {
	args := []string{
		"import.vmdk",
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		fmt.Sprintf("-pool=%s/Resources", creds.Cluster),
		fmt.Sprintf("-dc=%s", creds.Datacenter),
		fmt.Sprintf("-ds=%s", creds.Datastore),
		imagePath,
		imageName,
	}
	retcode := cli.Run(args)

	if retcode != 0 {
		return errors.New("importing vmdk failed")
	}
	return nil
}
