package vmware

import (
	"fmt"

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

// ImportVmdk is a function that uploads a stream optimized vmdk image to vSphere
// uploaded image will be present in a directory of the same name
func ImportVmdk(creds Credentials, imagePath string) error {
	args := []string{
		"import.vmdk",
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		fmt.Sprintf("-pool=%s/Resources", creds.Cluster),
		fmt.Sprintf("-dc=%s", creds.Datacenter),
		fmt.Sprintf("-ds=%s", creds.Datastore),
		imagePath,
	}
	retcode := cli.Run(args)

	if retcode != 0 {
		return fmt.Errorf("importing %s into vSphere failed", imagePath)
	}
	return nil
}

func ImportOva(creds Credentials, imagePath, targetName string) error {
	args := []string{
		"import.ova",
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		fmt.Sprintf("-pool=%s/Resources", creds.Cluster),
		fmt.Sprintf("-dc=%s", creds.Datacenter),
		fmt.Sprintf("-ds=%s", creds.Datastore),
		fmt.Sprintf("-name=%s", targetName),
		imagePath,
	}
	retcode := cli.Run(args)

	if retcode != 0 {
		return fmt.Errorf("importing %s into vSphere failed", imagePath)
	}
	return nil

}
