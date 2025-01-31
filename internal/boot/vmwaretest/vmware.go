//go:build integration

package vmwaretest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	// importing the packages registers these cli commands
	"github.com/vmware/govmomi/cli"
	_ "github.com/vmware/govmomi/cli/datastore"
	_ "github.com/vmware/govmomi/cli/importx"
	_ "github.com/vmware/govmomi/cli/vm"
	_ "github.com/vmware/govmomi/cli/vm/guest"
)

const WaitTimeout = 6000 // in seconds

type AuthOptions struct {
	Host       string
	Username   string
	Password   string
	Datacenter string
	Cluster    string
	Network    string
	Datastore  string
	Folder     string
}

func AuthOptionsFromEnv() (*AuthOptions, error) {
	host, hostExists := os.LookupEnv("GOVMOMI_URL")
	username, userExists := os.LookupEnv("GOVMOMI_USERNAME")
	password, pwdExists := os.LookupEnv("GOVMOMI_PASSWORD")
	datacenter, dcExists := os.LookupEnv("GOVMOMI_DATACENTER")
	cluster, clusterExists := os.LookupEnv("GOVMOMI_CLUSTER")
	network, netExists := os.LookupEnv("GOVMOMI_NETWORK")
	datastore, dsExists := os.LookupEnv("GOVMOMI_DATASTORE")
	folder, folderExists := os.LookupEnv("GOVMOMI_FOLDER")

	// If only one/two of them are not set, then fail
	if !hostExists {
		return nil, errors.New("GOVMOMI_URL not set")
	}

	if !userExists {
		return nil, errors.New("GOVMOMI_USERNAME not set")
	}

	if !pwdExists {
		return nil, errors.New("GOVMOMI_PASSWORD not set")
	}

	if !dcExists {
		return nil, errors.New("GOVMOMI_DATACENTER not set")
	}

	if !clusterExists {
		return nil, errors.New("GOVMOMI_CLUSTER not set")
	}

	if !netExists {
		return nil, errors.New("GOVMOMI_NETWORK not set")
	}

	if !dsExists {
		return nil, errors.New("GOVMOMI_DATASTORE not set")
	}

	if !folderExists {
		return nil, errors.New("GOVMOMI_FOLDER not set")
	}

	return &AuthOptions{
		Host:       host,
		Username:   username,
		Password:   password,
		Datacenter: datacenter,
		Cluster:    cluster,
		Network:    network,
		Datastore:  datastore,
		Folder:     folder,
	}, nil
}

func ImportImage(creds *AuthOptions, imagePath, imageName string) error {
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

func DeleteImage(creds *AuthOptions, directoryName string) error {
	retcode := cli.Run([]string{
		"datastore.rm",
		"-f=true",
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		fmt.Sprintf("-dc=%s", creds.Datacenter),
		fmt.Sprintf("-ds=%s", creds.Datastore),
		directoryName + "*", // because vm.create creates another directory with _1 prefix
	})

	if retcode != 0 {
		return errors.New("deleting directory failed")
	}
	return nil
}

func runWithStdout(args []string) (string, int) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	retcode := cli.Run(args)

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = oldStdout

	return strings.TrimSpace(string(out)), retcode
}

func WithBootedImage(creds *AuthOptions, imagePath, imageName, publicKey string, f func(address string) error) (retErr error) {
	vmdkBaseName := filepath.Base(imagePath)

	args := []string{
		"vm.create",
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		fmt.Sprintf("-pool=%s/Resources", creds.Cluster),
		fmt.Sprintf("-dc=%s", creds.Datacenter),
		fmt.Sprintf("-ds=%s", creds.Datastore),
		fmt.Sprintf("-folder=%s", creds.Folder),
		fmt.Sprintf("-net=%s", creds.Network),
		"-m=2048", "-g=rhel8_64Guest", "-on=true", "-firmware=efi",
		fmt.Sprintf("-disk=%s/%s", imageName, vmdkBaseName),
		"--disk.controller=scsi",
		imageName,
	}
	retcode := cli.Run(args)
	if retcode != 0 {
		return errors.New("Creating VM from vmdk failed")
	}

	defer func() {
		args = []string{
			"vm.destroy",
			fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
			"-k=true",
			imageName,
		}
		retcode := cli.Run(args)

		if retcode != 0 {
			fmt.Printf("Deleting VM %s failed", imageName)
			return
		}
	}()

	// note: by default this will wait/block until an IP address is returned
	// note: using exec() instead of running the command b/c .Run() returns an int
	args = []string{
		"vm.ip",
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		imageName,
	}
	ipAddress, retcode := runWithStdout(args)

	if retcode != 0 {
		return errors.New("Getting IP address for VM failed")
	}

	// Disabled b/c of https://github.com/vmware/govmomi/issues/2054
	// upload public key on the VM
	//args = []string{
	//	"guest.mkdir",
	//	fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
	//	"-k=true",
	//	fmt.Sprintf("-vm=%s", imageName),
	//	"-p", "/root/.ssh",
	//}
	//retcode = cli.Run(args)
	//if retcode != 0 {
	//	return errors.New("mkdir /root/.ssh on VM failed")
	//}

	//args = []string{
	//	"guest.upload",
	//	fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
	//	"-k=true",
	//	fmt.Sprintf("-vm=%s", imageName),
	//	"-f=true",
	//	publicKey, // this is a file path
	//	"/root/.ssh/authorized_keys",
	//}
	//retcode = cli.Run(args)
	//if retcode != 0 {
	//	return errors.New("Uploading public key to VM failed")
	//}

	return f(ipAddress)
}

// hard-coded SSH keys b/c we're having troubles uploading publicKey
// to the VM, see https://github.com/vmware/govmomi/issues/2054
func WithSSHKeyPair(f func(privateKey, publicKey string) error) error {
	public := "/usr/share/tests/osbuild-composer/keyring/id_rsa.pub"
	private := "/usr/share/tests/osbuild-composer/keyring/id_rsa"

	return f(private, public)
}
