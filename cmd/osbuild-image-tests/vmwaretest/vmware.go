// +build integration

package vmwaretest

import (
	"errors"
	"os"
	"os/exec"
	"fmt"
	"io/ioutil"
	"path/filepath"

	// importing the packages registers these cli commands
	"github.com/vmware/govmomi/govc/cli"
	_ "github.com/vmware/govmomi/govc/datastore"
	_ "github.com/vmware/govmomi/govc/importx"
	_ "github.com/vmware/govmomi/govc/vm"
	_ "github.com/vmware/govmomi/govc/vm/guest"
)

const WaitTimeout = 6000 // in seconds

type AuthOptions struct {
	Host string
	Username string
	Password string
	Datacenter string
	Cluster string
	Network string
	Datastore string
	Folder string
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

func WithBootedImage(creds *AuthOptions, imagePath, imageName, publicKey string, f func(address string) error) (retErr error) {
	vmdkBaseName := filepath.Base(imagePath)

	args := []string{
		"vm.create",
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		fmt.Sprintf("-dc=%s", creds.Datacenter),
		fmt.Sprintf("-ds=%s", creds.Datastore),
		fmt.Sprintf("-folder=%s", creds.Folder),
		fmt.Sprintf("-net=%s", creds.Network),
		"-m=2048", "-g=rhel8_64Guest", "-on=true", "-firmware=bios",
		fmt.Sprintf("-disk=%s/%s", imageName, vmdkBaseName),
		"--disk.controller=ide",
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
	ipAddress, err := exec.Command(
		"/usr/bin/govc",
		"vm.ip",
		fmt.Sprintf("-u=%s:%s@%s", creds.Username, creds.Password, creds.Host),
		"-k=true",
		imageName,
	).Output()
	if err != nil {
		fmt.Println("Getting IP address for VM failed:", string(ipAddress))
		return err
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

	return f(string(ipAddress))
}

// hard-coded SSH keys b/c we're having troubles uploading publicKey
// to the VM, see https://github.com/vmware/govmomi/issues/2054
func WithSSHKeyPair(f func(privateKey, publicKey string) error) error {
	public := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC61wMCjOSHwbVb4VfVyl5sn497qW4PsdQ7Ty7aD6wDNZ/QjjULkDV/yW5WjDlDQ7UqFH0Sr7vywjqDizUAqK7zM5FsUKsUXWHWwg/ehKg8j9xKcMv11AkFoUoujtfAujnKODkk58XSA9whPr7qcw3vPrmog680pnMSzf9LC7J6kXfs6lkoKfBh9VnlxusCrw2yg0qI1fHAZBLPx7mW6+me71QZsS6sVz8v8KXyrXsKTdnF50FjzHcK9HXDBtSJS5wA3fkcRYymJe0o6WMWNdgSRVpoSiWaHHmFgdMUJaYoCfhXzyl7LtNb3Q+Sveg+tJK7JaRXBLMUllOlJ6ll5Hod root@localhost"
	private := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABFwAAAAdzc2gtcn
NhAAAAAwEAAQAAAQEAutcDAozkh8G1W+FX1cpebJ+Pe6luD7HUO08u2g+sAzWf0I41C5A1
f8luVow5Q0O1KhR9Eq+78sI6g4s1AKiu8zORbFCrFF1h1sIP3oSoPI/cSnDL9dQJBaFKLo
7XwLo5yjg5JOfF0gPcIT6+6nMN7z65qIOvNKZzEs3/SwuyepF37OpZKCnwYfVZ5cbrAq8N
soNKiNXxwGQSz8e5luvpnu9UGbEurFc/L/Cl8q17Ck3ZxedBY8x3CvR1wwbUiUucAN35HE
WMpiXtKOljFjXYEkVaaEolmhx5hYHTFCWmKAn4V88pey7TW90Pkr3oPrSSuyWkVwSzFJZT
pSepZeR6HQAAA8gkYhqfJGIanwAAAAdzc2gtcnNhAAABAQC61wMCjOSHwbVb4VfVyl5sn4
97qW4PsdQ7Ty7aD6wDNZ/QjjULkDV/yW5WjDlDQ7UqFH0Sr7vywjqDizUAqK7zM5FsUKsU
XWHWwg/ehKg8j9xKcMv11AkFoUoujtfAujnKODkk58XSA9whPr7qcw3vPrmog680pnMSzf
9LC7J6kXfs6lkoKfBh9VnlxusCrw2yg0qI1fHAZBLPx7mW6+me71QZsS6sVz8v8KXyrXsK
TdnF50FjzHcK9HXDBtSJS5wA3fkcRYymJe0o6WMWNdgSRVpoSiWaHHmFgdMUJaYoCfhXzy
l7LtNb3Q+Sveg+tJK7JaRXBLMUllOlJ6ll5HodAAAAAwEAAQAAAQBbJjHNuLZ0lEfJvzF+
lu9hxqXVCl8rQPHszUBqGWMtXafNstKmBYBUCwzNJDN7YTisgrpRt3HViHPLYMpGvAQ9mV
bEpMYRdU0Z3Cqpv8XjZbtuhYC7OOn92SW7eOxAlZlD0hHuszOKtV9ayKWS8vZFVTB1yWhc
IyfYcK6vCdHUgPWrdiUJ7ULd0/t6thSCUYZxQAIBImDAh2GIKUV3b99WzUY8uAh4X70JoG
aVF2oFI/6gbIzvwUDqaEjzrll+ZRpxBdpQ+gdpGfvcKwipJrBEffd3Ji3TTqzqy91Iv/K9
Wm3ExbSe5JqMoimQkTf7BkTnNMViNzzFlW+yg9A2otUBAAAAgH+N7lqHL55QrDggHX3SmX
WzckNWvNP5q1rxLuy+WshivaFzXfihg7NWXpk3Jx8+Bi3AWP+6eKDjE2T6pEj+80dbeXOl
uoZOaRtFbfqMiPxa+UP+EeW8d5rb62U+gMbAVMM/0yQKCG5F6fU9tis34+ev0trR4DeWKS
n9yL/dkUQBAAAAgQDg4sL9BYI6GEz7JzBbww8Xc0zgIexf3LCFOiBPrw7Cb3uGOcjxMRnX
qf4LUeatYe/PCruhnLoCoHdaJc1JeXWjptfCefF0X0R2TeXdcLk0S9VY4vwk9FbbX9Wo6/
QS+SYr6b1MglUbvnFQpoGEZa8FaC7aKj+Y+k/+J32NwqEObQAAAIEA1LCzckxWUo9LvA11
7eNeK5VZLAjHabP6grsSgJugX6lQZ6hBnvB+J1w0IbXVxH5EMnl8zeVByWvK0B/XNTBSzw
S7NYXBuUG2if21fsJJB/9VW+UWXK8m8vpErnW5P+6RdichxRs9HuU41e3Y17DvxgiteQ5W
nQbQ6LErYhygDHEAAAAOcm9vdEBsb2NhbGhvc3QBAgMEBQ==
-----END OPENSSH PRIVATE KEY-----`

	return f(private, public)
}


func ConvertToStreamOptimizedVmdk(imagePath string) (string, error) {
	optimizedVmdk, err := ioutil.TempFile("/var/tmp", "osbuild-composer-stream-optimized-*.vmdk")
	if err != nil {
		return "", err
	}
	optimizedVmdk.Close()

	cmd := exec.Command(
		"/usr/bin/qemu-img", "convert", "-O", "vmdk", "-o", "subformat=streamOptimized",
		imagePath, optimizedVmdk.Name())
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	return optimizedVmdk.Name(), nil
}
