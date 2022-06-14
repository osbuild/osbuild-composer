package vmware

import (
	"errors"
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

// UploadImage is a function that uploads a stream optimized vmdk image to vSphere
// uploaded image will be present in a directory of the same name
func UploadImage(creds Credentials, imagePath string) error {
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
		return errors.New("importing vmdk failed")
	}
	return nil
}
