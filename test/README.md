# osbuild-composer testing information


## Integration testing

This will consume the osbuild-composer API surface via the `composer-cli`
command line interface. Implementation is under `cmd/osbuild-tests/`.

The easiest way to get started with integration testing from a git
checkout is:

* `dnf -y install rpm-build`
* `dnf -y builddep golang-github-osbuild-composer.spec`
* `make rpm` to build the software under test
* `dnf install output/x86_64/golang-github-osbuild-composer-*.rpm` - this will
  install both osbuild-composer, its -debuginfo, -debugsource and -tests packages
* `systemctl start osbuild-composer`
* `/usr/libexec/tests/osbuild-composer/osbuild-tests` to execute the test suite.
  It is best that you use a fresh system for installing and running the tests!

**NOTE:**

The easiest way to start osbuild-composer is via systemd because it takes care
of setting up the UNIX socket for the API server.

If you are working on a pull request that adds more integration tests
(without modifying osbuild-composer itself) then you can execute the test suite
from the local directory without installing it:

* `make build` - will build everything under `cmd/`
* `./osbuild-tests` - will execute the freshly built integration test suite
