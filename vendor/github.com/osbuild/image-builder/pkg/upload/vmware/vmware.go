package vmware

import (
	"fmt"

	"github.com/vmware/govmomi/cli"
	_ "github.com/vmware/govmomi/cli/importx"
)

type Credentials struct {
	Host       string
	Username   string
	Password   string
	Datacenter string
	Cluster    string
	Datastore  string
	Folder     string
}

func commonOptions(creds Credentials) []string {
	args := []string{
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		fmt.Sprintf("-pool=%s/Resources", creds.Cluster),
		fmt.Sprintf("-dc=%s", creds.Datacenter),
		fmt.Sprintf("-ds=%s", creds.Datastore),
	}
	if creds.Folder != "" {
		args = append(args, fmt.Sprintf("-folder=%s", creds.Folder))
	}

	return args
}

// ImportVmdk is a function that uploads a stream optimized vmdk image to vSphere
// uploaded image will be present in a directory of the same name
func ImportVmdk(creds Credentials, imagePath string) error {
	args := []string{
		"import.vmdk",
	}
	args = append(args, commonOptions(creds)...)
	args = append(args, imagePath)
	retcode := cli.Run(args)

	if retcode != 0 {
		return fmt.Errorf("importing %s into vSphere failed", imagePath)
	}
	return nil
}

func ImportOva(creds Credentials, imagePath, targetName string) error {
	args := []string{
		"import.ova",
		fmt.Sprintf("-name=%s", targetName),
	}
	args = append(args, commonOptions(creds)...)
	args = append(args, imagePath)
	retcode := cli.Run(args)

	if retcode != 0 {
		return fmt.Errorf("importing %s into vSphere failed", imagePath)
	}
	return nil

}
