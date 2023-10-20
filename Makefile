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
#     VERSION:
#         This evaluates the `Version` field of the specfile. Therefore, it will
#         be set to the latest version number of this repository without any
#         prefix (just a plain number).
#
#     COMMIT:
#         This evaluates to the latest git commit sha. This will not work if
#         the source is not a git checkout. Hence, this variable is not
#         pre-fetched but evaluated at time of use.
#

VERSION := $(shell (cd "$(SRCDIR)" && grep "^Version:" osbuild-composer.spec | sed 's/[^[:digit:]]*\([[:digit:]]\+\).*/\1/'))
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
	@echo "    unit-tests:         Run unit tests"

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
	- mkdir -p bin
	go build -o bin/osbuild-composer ./cmd/osbuild-composer/
	go build -o bin/osbuild-worker ./cmd/osbuild-worker/
	go build -o bin/osbuild-pipeline ./cmd/osbuild-pipeline/
	go build -o bin/osbuild-upload-azure ./cmd/osbuild-upload-azure/
	go build -o bin/osbuild-upload-aws ./cmd/osbuild-upload-aws/
	go build -o bin/osbuild-upload-gcp ./cmd/osbuild-upload-gcp/
	go build -o bin/osbuild-upload-oci ./cmd/osbuild-upload-oci/
	go build -o bin/osbuild-upload-generic-s3 ./cmd/osbuild-upload-generic-s3/
	go build -o bin/osbuild-mock-openid-provider ./cmd/osbuild-mock-openid-provider
	go build -o bin/osbuild-service-maintenance ./cmd/osbuild-service-maintenance
	go test -c -tags=integration -o bin/osbuild-composer-cli-tests ./cmd/osbuild-composer-cli-tests/main_test.go
	go test -c -tags=integration -o bin/osbuild-weldr-tests ./internal/client/
	go test -c -tags=integration -o bin/osbuild-dnf-json-tests ./cmd/osbuild-dnf-json-tests/main_test.go
	go test -c -tags=integration -o bin/osbuild-image-tests ./cmd/osbuild-image-tests/
	go test -c -tags=integration -o bin/osbuild-auth-tests ./cmd/osbuild-auth-tests/
	go test -c -tags=integration -o bin/osbuild-koji-tests ./cmd/osbuild-koji-tests/
	go test -c -tags=integration -o bin/osbuild-composer-dbjobqueue-tests ./cmd/osbuild-composer-dbjobqueue-tests/
	go test -c -tags=integration -o bin/osbuild-composer-maintenance-tests ./cmd/osbuild-service-maintenance/

.PHONY: install
install:
	- mkdir -p /usr/libexec/osbuild-composer
	cp bin/osbuild-composer /usr/libexec/osbuild-composer/
	cp bin/osbuild-worker /usr/libexec/osbuild-composer/
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

CERT_DIR=/etc/osbuild-composer

.PHONY: ca
ca:
ifneq (${CERT_DIR}/ca-key.pem${CERT_DIR}/ca-crt.pem,$(wildcard ${CERT_DIR}/ca-key.pem)$(wildcard ${CERT_DIR}/ca-crt.pem))
	@echo CA key or certificate file is missing, generating a new pair...
	- mkdir -p ${CERT_DIR}
	openssl req -new -nodes -x509 -days 365 -keyout ${CERT_DIR}/ca-key.pem -out ${CERT_DIR}/ca-crt.pem -subj "/CN=osbuild.org"
else
	@echo CA key and certificate files already exist, skipping...
endif

.PHONY: composer-key-pair
composer-key-pair: ca
	# generate a private key and a certificate request
	openssl req -new -nodes \
		-subj "/CN=localhost" \
		-keyout ${CERT_DIR}/composer-key.pem \
		-out ${CERT_DIR}/composer-csr.pem

	# sign the certificate
	openssl x509 -req \
		-in ${CERT_DIR}/composer-csr.pem \
		-CA ${CERT_DIR}/ca-crt.pem \
		-CAkey ${CERT_DIR}/ca-key.pem \
		-CAcreateserial \
		-out ${CERT_DIR}/composer-crt.pem

	# delete the request and set _osbuild-composer as the owner
	rm ${CERT_DIR}/composer-csr.pem
	chown _osbuild-composer:_osbuild-composer ${CERT_DIR}/composer-key.pem ${CERT_DIR}/composer-crt.pem

.PHONY: worker-key-pair
worker-key-pair: ca
	# generate a private key and a certificate request
	openssl req -new -nodes \
		-subj "/CN=localhost" \
		-keyout ${CERT_DIR}/worker-key.pem \
		-out ${CERT_DIR}/worker-csr.pem

	# sign the certificate
	openssl x509 -req \
		-in ${CERT_DIR}/worker-csr.pem \
		-CA ${CERT_DIR}/ca-crt.pem \
		-CAkey ${CERT_DIR}/ca-key.pem \
		-CAcreateserial \
		-out ${CERT_DIR}/worker-crt.pem

	# delete the request
	rm /etc/osbuild-composer/worker-csr.pem

.PHONY: unit-tests
unit-tests:
	go test -race ./...
	go test -race ./internal/dnfjson/... -force-dnf

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

RPM_SPECFILE=rpmbuild/SPECS/osbuild-composer.spec
RPM_TARBALL=rpmbuild/SOURCES/osbuild-composer-$(COMMIT).tar.gz

$(RPM_SPECFILE):
	mkdir -p $(CURDIR)/rpmbuild/SPECS
	git show HEAD:osbuild-composer.spec > $(RPM_SPECFILE)
	./tools/rpm_spec_add_provides_bundle.sh $(RPM_SPECFILE)

$(RPM_TARBALL):
	mkdir -p $(CURDIR)/rpmbuild/SOURCES
	git archive --prefix=osbuild-composer-$(COMMIT)/ --format=tar.gz HEAD > $(RPM_TARBALL)

.PHONY: srpm
srpm: $(RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bs \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--define "commit $(COMMIT)" \
		--with tests \
		$(RPM_SPECFILE)

.PHONY: rpm
rpm: $(RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--define "commit $(COMMIT)" \
		--with tests \
		$(RPM_SPECFILE)

.PHONY: scratch
scratch: $(RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--define "commit $(COMMIT)" \
		--without tests \
		--nocheck \
		$(RPM_SPECFILE)

