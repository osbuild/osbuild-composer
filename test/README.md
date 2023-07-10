# osbuild-composer testing information

With the exception of unit tests, all the osbuild-composer tests are shipped
in a dedicated RPM: osbulid-composer-tests, and they are meant to be installed
onto a pristine system and executed from there, rather than executed from a
git checkout.

Test cases are found in `test/cases`. They are intended to be independent of
each other and they are installed into `/usr/libexec/tests/osbuild-composer/`,
with any supporting test-data in `/usr/share/tests/osbuild-composer/`, and
helper binaries in `/usr/libexec/osbuild-composer-test/`.

Provisioning and orchestration of test-systems, as well as configuring what
distros, versions and architectures to test against is out of scope of the
tests themselves. For osbuild-composer CI, schutzbot is used, which can be
found in the directory by the same name.


## Golang tests

Test binaries, regardless of their scope/type (e.g. unit, API, integration)
that are written in Golang must follow the syntax of the Go
[testing package](https://golang.org/pkg/testing/), that is implement only
`TestXxx` functions with their setup/teardown when necessary in a `yyy_test.go`
file.

Test scenario discovery, execution and reporting will be handled by `go test`.

Some test files will be executed directly by `go test` during rpm build time
and/or in CI. These are usually unit tests. Scenarios which require more complex
setup, e.g. a running osbuild-composer are not intented to be executed directly
by `go test` at build time. Instead they are intended to be executed as
stand-alone test binaries on a clean system which has been configured in
advance (because this is easier/more feasible). These stand-alone test binaries
are also compiled via `go test -c -o` during rpm build or via `make build`.
See *Integration testing* for more information.

When comparing for expected values in test functions you should use the
[testify/assert](https://godoc.org/github.com/stretchr/testify/assert) or
[testify/require](https://godoc.org/github.com/stretchr/testify/require)
packages. Both of them provide an impressive array of assertions with the
possibility to use formatted strings as error messages. For example:

```
assert.Nilf(t, err, "Failed to set up temporary repository: %v", err)
```

If you want to fail immediately, not doing any more of the asserts use the
`require` package instead of the `assert` package, otherwise you'll end up with
panics and nil pointer memory problems.

Stand-alone test binaries also have the `-test.failfast` option.

Code coverage is recorded in
[codecov.io](https://codecov.io/github/osbuild/osbuild-composer).
This information comes only from unit tests and for the time being
we're not concerned with collecting coverage information from integration
tests, see `.github/workflows/tests.yml`.


## Image tests

In the `test/data/manifests` directory, sample image builds and their tests are
collected for the various distros, architectures, configuration we support.

Each test case describes how the image is built, the expected osbuild
manifest used internally, the expected image-info output and how to
boot-test the image.

To (re)generate these test cases use the tool
`tools/test-case-generators/generate-test-cases`.
Note that the `generate-test-cases` tool must be run on a host with
the same architecture, as the one intended for the generated test
cases. In other words, you need to generate e.g test cases for `aarch64`
images on an `aarch64` host.

These tests are executed from the `osbuild` repository in GitHub CI.

**Important Note:** `image-info` by default won't be able to read SELinux
labels used in the image, which are unknown to the host's policy. If you are
generating the image test case using
`tools/test-case-generators/generate-test-cases`, you'll have to relabel the
`image-info` tool with `osbuild_exec_t` in order to get correct report.

You can do this by running:
```bash
OSBUILD_LABEL=$(matchpathcon -n $(which osbuild))
chcon $OSBUILD_LABEL tools/image-info
```

### Generating all test cases

To (re)generate test cases for all architectures, or just
the ones different from your host's architecture, you can use the
`tools/test-case-generators/generate-all-test-cases` script.

The script generates image test cases in so-called Runner, which is a system
of a specific architecture. The used Runner type is specific to the used
command, but in general it is a system accessible via SSH connection.

In simplified example, the script does the following:

1. Provisions Runners if needed.
2. Waits for the Runner to be ready for use by running a specific command n it.
3. Installs RPMs necessary for the test case generation on the Runner.
    - In case you need to install packages from a specific external repository, you can specify each such repository using `--repofrompath` option. For example if you want to use the latest `osbuild` upstream build, use `--repofrompath 'osbuild,https://download.copr.fedorainfracloud.org/results/@osbuild/osbuild/fedora-$releasever-$basearch/'`.
    - In case you need to install osbuild-composer RPMs, which were built from the sources copied over to the runner, use the `--build-rpms` option. The script will build osbuild-composer RPMs on the remote runner and install them.
4. Copies the 'sources' using rsync to the Runner.
5. Executes the 'tools/test-case-generators/generate-test-cases' on the runner for each requested distro and image type.
6. After each image test case is generated successfully, the result is copied using rsync from the Runner to 'output' directory.

The script by default generates all image test cases defined in
`tools/test-case-generators/distro-arch-imagetype-map.json`. Unless you want to
reduce the matrix of generated test cases, you don't need to specify any of
`--arch`, `--distro` or `--image-type` options. These only filter the default
matrix. So to e.g. generate all image test cases for RHEL-8.5, simply run:

```bash
$ ./tools/test-case-generators/generate-all-test-cases \
        --output test/data/manifests \
        --distro rhel-85 \
        <COMMAND> \
        ...
```

The script supports the following commands:

- `qemu` - generates image test cases locally using QEMU VMs.
- `remote` - generates image test cases on existing remote hosts. This command does not use QEMU on the remote host. It executes commands directly on the remote system.

**Generating test cases in QEMU example:**

```bash
$ ./tools/test-case-generators/generate-all-test-cases \
        --output test/data/manifests \
        --arch aarch64 \
        --arch s390x \
        --arch ppc64le \
        --distro rhel-8 \
        --image-type qcow2 \
        qemu \
        --image-x86_64 ~/Downloads/Images/Fedora-Cloud-Base-33-1.2.x86_64.qcow2 \
        --image-ppc64le ~/Downloads/Images/Fedora-Cloud-Base-33-1.2.ppc64le.qcow2 \
        --image-aarch64 ~/Downloads/Images/Fedora-Cloud-Base-33-1.2.aarch64.qcow2 \
        --image-s390x ~/Downloads/Images/Fedora-Cloud-Base-33-1.2.s390x.qcow2
```

**Generating test cases using existing remote hosts example:**

```bash
$ ./tools/test-case-generators/generate-all-test-cases \
        --output test/data/manifests \
        --arch aarch64 \
        --arch s390x \
        --arch ppc64le \
        --distro rhel-8 \
        --image-type qcow2 \
        remote \
        --host-ppc64le 192.168.1.10 \
        --host-aarch64 192.168.1.20 \
        --host-s390x 192.168.1.30
```

### Setting up Azure upload tests

By default, the vhd images are run locally using qemu. However, when
the right set of environment flags is passed to the osbuild-image-tests,
it uploads the image to Azure, boots it and tries to ssh into it.

#### Required flags
- `AZURE_STORAGE_ACCOUNT`
- `AZURE_STORAGE_ACCESS_KEY`
- `AZURE_CONTAINER_NAME`
- `AZURE_SUBSCRIPTION_ID`
- `V2_AZURE_CLIENT_ID`
- `V2_AZURE_CLIENT_SECRET`
- `AZURE_TENANT_ID`
- `AZURE_LOCATION`
- `AZURE_RESOURCE_GROUP`

#### Setting up all the required resources

1) Firstly, go to *Subscriptions* in the left-side menu. Here you can find
   the `AZURE_SUBSCRIPTION_ID`.

2) Now, you need to create a new resource group. In the left-side menu,
   select *Resource groups*. Click on *Add* above the resource group list.
   The name you choose is your `AZURE_RESOURCE_GROUP`. The region you choose
   is your `AZURE_LOCATION`. However, it must be in the "machine-readable
   form". You can list all the locations with their machine-readable names
   using Azure CLI: `az account list-locations -o table`.
   E.g. the machine-readable name of US East location is `eastus`.
   
   Note that terms *location* and *region* are synonyms in Azure's context.

3) Storage time! Go to Storage accounts in the left-side menu. Click on
   *Add* above the list. Use the resource group you created in
   the previous step. Also, the region should be the same. The name you
   choose is your `AZURE_STORAGE_ACCOUNT`.

   After the storage account is created, open it.
   Select *Settings > Access keys*. Choose one of the keys, this is your
   `AZURE_STORAGE_ACCESS_KEY`. Select *Blob service > Containers* and create
   a new one. Its name is your `AZURE_CONTAINER_NAME`.

4) Now it’s time to create an application. This is needed because Azure uses
   OAuth to do authorization. In the left-side menu, choose *Azure Active
   Directory*. Go to *Manage > App registrations* and register a new
   application.

   When it’s created, open it. In the overview, you can see
   the Application (client) ID and the Directory (tenant) ID. These are your
   `V2_AZURE_CLIENT_ID` and `AZURE_TENANT_ID`.

   Now, go to *Manage > Certificates & Secrets* under your new application
   and create a new client secret. The is your `V2_AZURE_CLIENT_SECRET`.

5) The last step is to give the new application access to the resource group.
   This step must be done by Azure administrator (@larskarlitski): Go to
   the *Access control (IAM)* section under the newly created resource group.
   Here, add the new application with the *Developer* role.

### Setting up GCP upload tests

The following environment variables are required

- `GCP_BUCKET`
- `GCP_REGION`
- `GOOGLE_APPLICATION_CREDENTIALS`

### Setting up OpenStack upload tests

The following environment variables are required

- `OS_AUTH_URL`
- `OS_USERNAME`
- `OS_PASSWORD`
- `OS_PROJECT_ID`
- `OS_DOMAIN_NAME`


### Setting up VMware vCenter upload tests

The following environment variables are required

- `GOVC_URL` - vCenter hostname
- `GOVC_USERNAME`
- `GOVC_PASSWORD`
- `GOVC_DATACENTER`
- `GOVC_CLUSTER`
- `GOVC_NETWORK`
- `GOVC_DATASTORE`
- `GOVC_FOLDER`
- `GOVC_INSECURE` - value of 1 will skip checking SSL certificates

**WARNING:** when configuring the credentials for Schutzbot we've experienced
an issue where the first line in the credentials file gets lost resulting in
incomplete credentials. The work-around is to define a dummy ENV variable on
the first line!

### Cloud cleaner

[Cloud cleaner](https://github.com/osbuild/cloud-cleaner) is a tool designed to clean leftover cloud resources in order to reduce our costs.

Some tests deploy images to different clouds. After the tests run, these images get removed by the same tests. But if something fails during these, or a pipeline gets canceled, those images could get left behind in the cloud, wasting resources.

Cloud cleaner is being executed every hour (on the CI of it's own repo), and it cleans resources that are not tagged with `persist=true`

## Integration testing

Since `osbuild-composer` externally provides two types of API, there also
multiple types of integration tests available.

Types of APIs tested:
- *Weldr API* - the original API provided by `lorax-composer`, for which
  `osbuild-composer` is a drop-in replacement. This API is mostly tested through
  the `composer-cli` tool, which consumes the *Weldr API*.
- *Cloud API* - the brand new API provided by `osbuild-composer`, which is
  currently used by the *Image Builder* service.

### Weldr API integration testing

This will consume the osbuild-composer *Weldr API* surface via the `composer-cli`
command line interface. Implementation is under
`cmd/osbuild-composer-cli-tests/`.

The easiest way to get started with integration testing from a git
checkout is:

* `dnf -y install rpm-build`
* `dnf -y builddep osbuild-composer.spec`
* `make rpm` to build the software under test
* `dnf install rpmbuild/RPMS/x86_64/osbuild-composer-*.rpm` - this will
  install both osbuild-composer, its -debuginfo, -debugsource and -tests packages
* `systemctl start osbuild-composer`
* `/usr/libexec/osbuild-composer-test/osbuild-composer-cli-tests` to execute
  the test suite.

It is best that you use a fresh system for installing and running the tests!

**NOTE:**

The easiest way to start osbuild-composer is via systemd because it takes care
of setting up the UNIX socket for the API server.

If you are working on a pull request that adds more integration tests
(without modifying osbuild-composer itself) then you can execute the test suite
from the local directory without installing it:

* `make build` - will build everything under `cmd/`
* `./osbuild-composer-cli-tests` - will execute the freshly built integration test suite

### Cloud API integration testing

*Cloud API* integration tests use the new REST API of `osbuild-composer` to request
an image build for a selected footprint. If the target is a public cloud environment
then the image is uploaded to the specific cloud provider as part of the compose.

The easiest way to get started with integration testing from a git
checkout is:

* `dnf -y install rpm-build`
* `dnf -y builddep osbuild-composer.spec`
* `make rpm` to build the software under test
* `dnf install rpmbuild/RPMS/x86_64/osbuild-composer-*.rpm` - this will
  install both osbuild-composer, its -debuginfo, -debugsource and -tests packages
* `systemctl start osbuild-composer`
* `/usr/libexec/tests/osbuild-composer/api.sh <CLOUD_PROVIDER>` to execute
  the integration testing with a specific cloud provider. Valid `CLOUD_PROVIDER`
  values are `aws` and `gcp`.

It is best that you use a fresh system for installing and running the tests!

The *Cloud API* integration testing usually consists from:

- Test existing *Cloud API* endpoints and their responses.
- Request a new compose.
- Test uploading of the image to specific cloud provider and sharing
  of the image with specified accounts.
- Optionally there are additional tests performed on the image, such as:
  - Create a VM instance with the cloud provider and uploaded image.
  - Run various checks on the VM instance.
- The last step is cleanup of resources uploaded and created with the cloud provider
  as part of the integration test.

#### Setting up AWS integration tests

The following environment variables are required

- `AWS_REGION`
- `AWS_BUCKET`
- `V2_AWS_ACCESS_KEY_ID`
- `V2_AWS_SECRET_ACCESS_KEY`
- `AWS_API_TEST_SHARE_ACCOUNT`

To execute the AWS integration tests, complete steps from *Cloud API integration testing*
section and run `/usr/libexec/tests/osbuild-composer/api.sh aws`.

#### Setting up GCP integration tests

The following environment variables are required:

- `GOOGLE_APPLICATION_CREDENTIALS` - path to [Google authentication credentials][gcp_creds] file.
- `GCP_REGION`
- `GCP_BUCKET`
- `GCP_API_TEST_SHARE_ACCOUNT`

To execute the GCP integration tests, complete steps from *Cloud API integration testing*
section and run `/usr/libexec/tests/osbuild-composer/api.sh gcp`.

[gcp_creds]: https://cloud.google.com/docs/authentication/getting-started#setting_the_environment_variable

## Downstream testing notes

To make it easier for us to test & verify downstream builds we are going to
move most of the work upstream and apply the following rules:

1. Preferably the 1st commit of any PR will contain a bug reproducer.
   First push a draft PR only containing that commit which will cause CI to FAIL.
2. QE will review and approve the reproducer (can happen in parallel with next item)
3. Subsequent commits provide bug fixes without modifying the reproducer and
   CI reports PASS. Push these on top of the approved reproducer.
4. QE has done final review and approved the PR; RHBZ status is set to
   `MODIFIED + Verified=Tested`
5. Devel has done final review and approved the PR

**NOTES for devel:**

Pull requests related to new functionality may add their
automated tests together or after commit(s) adding said functionality!

All PRs containing commits referencing `rhbz#` number and/or
all PRs against a dedicated `rhel-` branch should follow the above rules!

**NOTE for QE:**

CI results are also reported against each commit and these
can be used to review the test automation state during a PR lifecycle.

`qa_ack+` on RHBZ will be granted **after** a reproducer has been
identified and with the mutual understanding that PRs related to
that RHBZ must include an automated test reproducer. A good example is
[BZ #1951192](https://bugzilla.redhat.com/show_bug.cgi?id=1951192#c3) while
a bad example is
[BZ #1942029](https://bugzilla.redhat.com/show_bug.cgi?id=1942029#c0) b/c
is is missing lots of details!


## QE associates/responsibilities matrix

- [@atodorov](https://github.com/atodorov) - QE lead;
  osbuild, osbuild-composer, weldr-client, composer-cli, image-builder, image-builder-frontend,
  bug testing & verification, anything else
- [@jrusz](https://github.com/jrusz) - testing across the board; backup for @atodorov;
  osbuild, osbuild-composer, weldr-client, composer-cli, image-builder, image-builder-frontend,
  bug testing & verification, anything else
- [@henrywang](https://github.com/henrywang) - everything related to testing RHEL for Edge images
- [@yih-redhat](https://github.com/yih-redhat) - everything related to testing RHEL for Edge images;
  backup for @henrywang


## Internal build testing in Gitlab CI

In Gitlab CI we're using rules to define which stages and jobs are ran when. For running the test
suite on internal RHEL builds we're using schedules that run the test suite on latest nightly using
the rpms that are shipped with it.

We're using a link to latest nightly build. It's possible to manually override it with `COMPOSE_URL` 
to use a different build instead.
This variable can be changed in the pipeline schedule settings. It is advised to create
a new schedule if you want to try something or change the value, set it as inactive and then
just trigger it manually.

   !['Create pipeline schedule'](./gitlab_ci_scheduled_pipeline.png)

   !['Trigger scheduled pipeline'](./gitlab_ci_schedule_trigger.png)
