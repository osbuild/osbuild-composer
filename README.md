# osbuild-composer

An HTTP service for building bootable OS images. It provides the same API as [lorax-composer](https://github.com/weldr/lorax) but in the background it uses [osbuild](https://github.com/osbuild/osbuild) to create the images.

You can control it in [Cockpit](https://github.com/weldr/cockpit-composer) or using the [composer-cli](https://weldr.io/lorax/composer-cli.html). To get started on Fedora, run:

```
# dnf install cockpit-composer golang-github-osbuild-composer composer-cli
# systemctl enable --now cockpit.socket
# systemctl enable --now osbuild-composer.socket
```

Now you can access the service using `composer-cli`, for example:

```
composer-cli status show
```

or using a browser: `http://localhost:9090`

## API documentation

Please refer to the [lorax-composer](https://github.com/weldr/lorax)'s documenation as osbuild-composer is a drop-in replacement.

## High-level overview

![overview](docs/osbuild-composer.svg)

### Frontends

`osbuild-composer` is meant to be used with 2 different front-ends. The primary one, which is meant for general use, is cockpit-composer. It is part of the Cockpit project and unless you have a strong reason not to use it, you should use it. `composer-cli` is a command line tool that can be used with `osbuild-composer`.

### Compose
* Compose is what the user submits over one of the frontends
* It contains of one or more **image builds**
* It contains zero or more **upload actions**

### Image build
* The resulting *image* has a *type*: https://github.com/osbuild/osbuild-composer/blob/master/internal/distro/fedora30/distro.go#L19
* Running build in osbuild-composer is referred to as a "job" (internal terminology, not related to end-user experience)

### Job
* What composer submits to a worker
* Is a unit of work performed by `osbuild` (internally it is a single execution of `osbuild`)
* Consists of **one** image build and **zero or more** Upload actions

### Image type
* In the cockpit-composer, for examples these are image types:
  * Openstack
  * Azure
  * AWS
* As of now, we name them internally by their file format: vhd, ami, etc.
* You can see a list of types by executing: `composer-cli compose types`

### Upload action
* Each image can be, but does not have to be, uploaded to a remote location
* One image can be uploaded to multiple locations


## Testing

See [test/README.md](test/README.md)

## Running the development VM
Unfortunately, you cannot run osbuild-composer in a container due to the fact that osbuild requires /dev to be able to create loop devices. Therefore, we have the following vm setup.


### Prerequisites
You need to have vagrant and rpm build tools installed:
```
sudo dnf install -y rpm-build 'dnf-command(builddep)' vagrant
```

Next, you need to install build dependencies of osbuild and osbuild-composer by running the following command inside the cloned osbuild-composer repository:

```
wget https://raw.githubusercontent.com/osbuild/osbuild-composer/master/golang-github-osbuild-composer.spec
sudo dnf builddep -y osbuild.spec golang-github-osbuild-osbuild.composer.spec
rm osbuild.spec
```

### Provisioning the VM
The following command will provision a VM with osbuild and cockpit-composer installed. Cockpit-composer and its dependencies are installed from the repositories, osbuild is installed from rpms built on your local machine from the current master branch.
```
make vm-provision
``` 

*Note: This command will delete any previous vagrant VMs created in this directory.*

### Installing osbuild-composer from source
The following command will install osbuild-composer from your local git checkout. Firstly, rpms are locally built, then they are rsynced to the VM and finally they're installed in the VM.
```
make vm-install
```

### Running tests inside the VM
After the VM is provisioned and osbuild-composer is installed, you can run the integration test suite using the following command:
```
make vm-test
```
### Accessing the VM
You have two options to access the VM:

1) Using ssh, just run `vagrant ssh`
2) Using cockpit, go to http://localhost:9091 and log in as admin/foobar 
