# Contributing to osbuild-composer

First of all, thank you for taking the time to contribute to osbuild-composer.
In this document you will find information that can help you with your
contribution.
For more information feel free to read our [developer guide](https://www.osbuild.org/docs/developer-guide/index).

### Running from sources

We recommend using the latest stable Fedora for development but latest minor
release of RHEL/CentOS 8 should work just fine as well. To run osbuild-composer
from sources, follow these steps:

1. Clone the repository and create a local checkout:

```
$ git clone https://github.com/osbuild/osbuild-composer.git
$ cd osbuild-composer
```

2. To install the build-requirements for Fedora and friends, use:

```
$ sudo dnf group install -y 'RPM Development Tools'   # Install rpmbuild
$ sudo dnf builddep -y osbuild-composer.spec          # Install build-time dependencies
$ sudo dnf -y install cockpit-composer             # Optional: Install cockpit integration
$ sudo systemctl start cockpit.socket              # Optional: Start cockpit
```

3. Finally, use the following to compile the project from the working
directory, install it, and then run it:

```
$ rm -rf rpmbuild/
$ make rpm
$ sudo dnf -y install rpmbuild/RPMS/x86_64/*
$ sudo systemctl start osbuild-composer.socket
```

You can now open https://localhost:9090 in your browser to open cockpit console
and check the "Image Builder" section.

Alternatively you can use `composer-cli` to interact with the Weldr API. We
don't have any client for the RCM API, so the only option there is a
plain `curl`.

When developing the code, use `go` executable to generate, build, run, and test you
code [1], alternatively you can use the script `tools/prepare-source.sh`:

```
$ go test ./...
$ go build ./...
$ go generate ./...
```

### Testing

See [test/README.md](test/README.md) for more information about testing.

### Planning the work

In general we encourage you to first fill in an issue and discuss the feature
you would like to work on before you start. This can prevent the scenario where
you work on something we don't want to include in our code.

That being said, you are of course welcome to implement an example of what you
would like to achieve.

### Creating a PR

The commits in the PR should have these properties:

* Preferably minimal and well documented
  * Where minimal means: don't do unrelated changes even if the code is
    obviously wrong
  * Well documented: both code and commit message
  * The commit message should start with the module you work on,
    like: `weldr: `, or `distro:`
* The code should compile and the composer should start, so that we can run
  `git bisect` on it
* All code should be covered by unit-tests

### Notes:

[1] If you are running macOS, you can still compile osbuild-composer. If it
    doesn't work out of the box, use `-tags macos` together with any `go`
    command, for example: `go test -tags macos ./...`
