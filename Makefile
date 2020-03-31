#
# Maintenance Helpers
#
# This makefile contains targets used for development, as well as helpers to
# aid automatization of maintenance. Unless a target is documented in
# `make help`, it is not supported and is only meant to be used by developers
# to aid their daily development work.
#
# All supported targets honor the `SRCDIR` variable to find the source-tree.
# For most unsupported targets, you are expected to have the source-tree as
# your working directory. To specify a different source-tree, simply override
# the variable via `SRCDIR=<path>` on the commandline. By default, the working
# directory is used for build output, but `BUILDDIR=<path>` allows overriding
# it.
#

BUILDDIR ?= .
SRCDIR ?= .

RST2MAN ?= rst2man

#
# Automatic Variables
#
# This section contains a bunch of automatic variables used all over the place.
# They mostly try to fetch information from the repository sources to avoid
# hard-coding them in this makefile.
#
# Most of the variables here are pre-fetched so they will only ever be
# evaluated once. This, however, means they are always executed regardless of
# which target is run.
#
#     COMMIT:
#         This evaluates to the latest git commit sha. This will not work if
#         the source is not a git checkout. Hence, this variable is not
#         pre-fetched but evaluated at time of use.
#

COMMIT = $(shell (cd "$(SRCDIR)" && git rev-parse HEAD))

#
# Generic Targets
#
# The following is a set of generic targets used across the makefile. The
# following targets are defined:
#
#     help
#         This target prints all supported targets. It is meant as
#         documentation of targets we support and might use outside of this
#         repository.
#         This is also the default target.
#
#     $(BUILDDIR)/
#     $(BUILDDIR)/%/
#         This target simply creates the specified directory. It is limited to
#         the build-dir as a safety measure. Note that this requires you to use
#         a trailing slash after the directory to not mix it up with regular
#         files. Lastly, you mostly want this as order-only dependency, since
#         timestamps on directories do not affect their content.
#

.PHONY: help
help:
	@echo "make [TARGETS...]"
	@echo
	@echo "This is the maintenance makefile of osbuild. The following"
	@echo "targets are available:"
	@echo
	@echo "    help:               Print this usage information."
	@echo "    man:                Generate all man-pages"

$(BUILDDIR)/:
	mkdir -p "$@"

$(BUILDDIR)/%/:
	mkdir -p "$@"

#
# Documentation
#
# The following targets build the included documentation. This includes the
# packaged man-pages, but also all other kinds of documentation that needs to
# be generated. Note that these targets are relied upon by automatic
# deployments to our website, as well as package manager scripts.
#

MANPAGES_RST = $(wildcard $(SRCDIR)/docs/*.[0123456789].rst)
MANPAGES_TROFF = $(patsubst $(SRCDIR)/%.rst,$(BUILDDIR)/%,$(MANPAGES_RST))

$(MANPAGES_TROFF): $(BUILDDIR)/docs/%: $(SRCDIR)/docs/%.rst | $(BUILDDIR)/docs/
	$(RST2MAN) "$<" "$@"

.PHONY: man
man: $(MANPAGES_TROFF)

#
# Maintenance Targets
#
# The following targets are meant for development and repository maintenance.
# They are not supported nor is their use recommended in scripts.
#

.PHONY: build
build:
	go build -o osbuild-composer ./cmd/osbuild-composer/
	go build -o osbuild-worker ./cmd/osbuild-worker/
	go build -o osbuild-pipeline ./cmd/osbuild-pipeline/
	go build -o osbuild-upload-azure ./cmd/osbuild-upload-azure/
	go build -o osbuild-upload-aws ./cmd/osbuild-upload-aws/
	go test -c -tags=integration -o osbuild-tests ./cmd/osbuild-tests/main_test.go
	go test -c -tags=integration -o osbuild-weldr-tests ./internal/client/
	go test -c -tags=integration -o osbuild-dnf-json-tests ./cmd/osbuild-dnf-json-tests/main_test.go
	go test -c -tags=integration -o osbuild-rcm-tests ./cmd/osbuild-rcm-tests/main_test.go
	go test -c -tags=integration,travis -o osbuild-image-tests ./cmd/osbuild-image-tests/

.PHONY: install
install:
	- mkdir -p /usr/libexec/osbuild-composer
	cp osbuild-composer /usr/libexec/osbuild-composer/
	cp osbuild-worker /usr/libexec/osbuild-composer/
	cp dnf-json /usr/libexec/osbuild-composer/
	- mkdir -p /usr/share/osbuild-composer/repositories
	cp repositories/* /usr/share/osbuild-composer/repositories
	- mkdir -p /etc/sysusers.d/
	cp distribution/osbuild-composer.conf /etc/sysusers.d/
	systemd-sysusers osbuild-composer.conf
	- mkdir -p /etc/systemd/system/
	cp distribution/*.service /etc/systemd/system/
	cp distribution/*.socket /etc/systemd/system/
	systemctl daemon-reload

.PHONY: ca
ca:
ifneq (/etc/osbuild-composer/ca-key.pem/etc/osbuild-composer/ca-crt.pem,$(wildcard /etc/osbuild-composer/ca-key.pem)$(wildcard /etc/osbuild-composer/ca-crt.pem))
	@echo CA key or certificate file is missing, generating a new pair...
	- mkdir -p /etc/osbuild-composer
	openssl req -new -nodes -x509 -days 365 -keyout /etc/osbuild-composer/ca-key.pem -out /etc/osbuild-composer/ca-crt.pem -subj "/CN=osbuild.org"
else
	@echo CA key and certificate files already exist, skipping...
endif

.PHONY: composer-key-pair
composer-key-pair: ca
	openssl genrsa -out /etc/osbuild-composer/composer-key.pem 2048
	openssl req -new -sha256 -key /etc/osbuild-composer/composer-key.pem	-out /etc/osbuild-composer/composer-csr.pem -subj "/CN=localhost" # TODO: we need to generate certificates with another hostname
	openssl x509 -req -in /etc/osbuild-composer/composer-csr.pem  -CA /etc/osbuild-composer/ca-crt.pem -CAkey /etc/osbuild-composer/ca-key.pem -CAcreateserial -out /etc/osbuild-composer/composer-crt.pem
	chown _osbuild-composer:_osbuild-composer /etc/osbuild-composer/composer-key.pem /etc/osbuild-composer/composer-csr.pem /etc/osbuild-composer/composer-crt.pem

.PHONY: worker-key-pair
worker-key-pair: ca
	openssl genrsa -out /etc/osbuild-composer/worker-key.pem 2048
	openssl req -new -sha256 -key /etc/osbuild-composer/worker-key.pem	-out /etc/osbuild-composer/worker-csr.pem -subj "/CN=localhost"
	openssl x509 -req -in /etc/osbuild-composer/worker-csr.pem  -CA /etc/osbuild-composer/ca-crt.pem -CAkey /etc/osbuild-composer/ca-key.pem -CAcreateserial -out /etc/osbuild-composer/worker-crt.pem


#
# Building packages
#
# The following rules build osbuild-composer packages from the current HEAD
# commit, based on the spec file in this directory.  The resulting packages
# have the commit hash in their version, so that they don't get overwritten
# when calling `make rpm` again after switching to another branch.
#
# All resulting files (spec files, source rpms, rpms) are written into
# ./rpmbuild, using rpmbuild's usual directory structure.
#

OLD_RPM_SPECFILE=rpmbuild/SPECS/golang-github-osbuild-composer-$(COMMIT).spec
RPM_SPECFILE=rpmbuild/SPECS/osbuild-composer-$(COMMIT).spec
RPM_TARBALL=rpmbuild/SOURCES/osbuild-composer-$(COMMIT).tar.gz

$(OLD_RPM_SPECFILE):
	mkdir -p $(CURDIR)/rpmbuild/SPECS
	(echo "%global commit $(COMMIT)"; git show HEAD:golang-github-osbuild-composer.spec) > $(OLD_RPM_SPECFILE)

$(RPM_SPECFILE):
	mkdir -p $(CURDIR)/rpmbuild/SPECS
	(echo "%global commit $(COMMIT)"; git show HEAD:osbuild-composer.spec) > $(RPM_SPECFILE)

$(RPM_TARBALL):
	mkdir -p $(CURDIR)/rpmbuild/SOURCES
	git archive --prefix=osbuild-composer-$(COMMIT)/ --format=tar.gz HEAD > $(RPM_TARBALL)

.PHONY: srpm
srpm: $(RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bs \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--with tests \
		$(RPM_SPECFILE)

.PHONY: rpm
rpm: $(RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--with tests \
		$(RPM_SPECFILE)

.PHONY: old-srpm
old-srpm: $(OLD_RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bs \
		--define "_topdir $(CURDIR)/rpmbuild" \
		$(OLD_RPM_SPECFILE)

.PHONY: old-rpm
old-rpm: $(OLD_RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		$(OLD_RPM_SPECFILE)
