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

# see https://hub.docker.com/r/docker/golangci-lint/tags
# v1.55 to get golang 1.21 (1.21.3)
# v1.53 to get golang 1.20 (1.20.5)
GOLANGCI_LINT_VERSION=v1.55
GOLANGCI_LINT_CACHE_DIR=$(HOME)/.cache/golangci-lint/$(GOLANGCI_LINT_VERSION)
GOLANGCI_COMPOSER_IMAGE=composer_golangci
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
	@echo "This is the maintenance makefile of osbuild-composer. The following"
	@echo "targets are available:"
	@echo
	@echo "    help:               Print this usage information."
	@echo "    build:              Build all binaries"
	@echo "    rpm:                Build the RPM"
	@echo "    srpm:               Build the source RPM"
	@echo "    scratch:            Quick scratch build of RPM"
	@echo "    clean:              Remove all built binaries"
	@echo "    man:                Generate all man-pages"
	@echo "    unit-tests:         Run unit tests"
	@echo "    push-check:         Replicates the github workflow checks as close as possible"
	@echo "                        (do this before pushing!)"
	@echo "    lint:               Runs linters as close as github workflow as possible"
	@echo "    process-templates:  Execute the OpenShift CLI to check the templates"
	@echo "    coverage-report:    Run unit tests and generate HTML coverage reports"

$(BUILDDIR)/:
	mkdir -p "$@"

$(BUILDDIR)/%/:
	mkdir -p "$@"

$(GOLANGCI_LINT_CACHE_DIR):
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
build: $(BUILDDIR)/bin/
	go build -o $<osbuild-composer ./cmd/osbuild-composer/
	go build -o $<osbuild-worker ./cmd/osbuild-worker/
	go build -o $<osbuild-worker-executor ./cmd/osbuild-worker-executor/
	go build -o $<osbuild-upload-azure ./cmd/osbuild-upload-azure/
	go build -o $<osbuild-upload-aws ./cmd/osbuild-upload-aws/
	go build -o $<osbuild-upload-gcp ./cmd/osbuild-upload-gcp/
	go build -o $<osbuild-upload-oci ./cmd/osbuild-upload-oci/
	go build -o $<osbuild-upload-generic-s3 ./cmd/osbuild-upload-generic-s3/
	go build -o $<osbuild-mock-openid-provider ./cmd/osbuild-mock-openid-provider
	go build -o $<osbuild-service-maintenance ./cmd/osbuild-service-maintenance
	go build -o $<osbuild-jobsite-manager ./cmd/osbuild-jobsite-manager
	go build -o $<osbuild-jobsite-builder ./cmd/osbuild-jobsite-builder
	go test -c -tags=integration -o $<osbuild-composer-cli-tests ./cmd/osbuild-composer-cli-tests/main_test.go
	go test -c -tags=integration -o $<osbuild-weldr-tests ./internal/client/
	go test -c -tags=integration -o $<osbuild-dnf-json-tests ./cmd/osbuild-dnf-json-tests/main_test.go
	go test -c -tags=integration -o $<osbuild-image-tests ./cmd/osbuild-image-tests/
	go test -c -tags=integration -o $<osbuild-auth-tests ./cmd/osbuild-auth-tests/
	go test -c -tags=integration -o $<osbuild-koji-tests ./cmd/osbuild-koji-tests/
	go test -c -tags=integration -o $<osbuild-composer-dbjobqueue-tests ./cmd/osbuild-composer-dbjobqueue-tests/
	go test -c -tags=integration -o $<osbuild-composer-maintenance-tests ./cmd/osbuild-service-maintenance/

.PHONY: install
install: build
	- mkdir -p /usr/libexec/osbuild-composer
	cp $(BUILDDIR)/bin/osbuild-composer /usr/libexec/osbuild-composer/
	cp $(BUILDDIR)/bin/osbuild-worker /usr/libexec/osbuild-composer/
	- mkdir -p /usr/share/osbuild-composer/repositories
	cp repositories/* /usr/share/osbuild-composer/repositories
	- mkdir -p /etc/sysusers.d/
	cp distribution/osbuild-composer.conf /etc/sysusers.d/
	systemd-sysusers osbuild-composer.conf
	- mkdir -p /etc/systemd/system/
	cp distribution/*.service /etc/systemd/system/
	cp distribution/*.socket /etc/systemd/system/
	systemctl daemon-reload

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)/bin/
	rm -rf $(CURDIR)/rpmbuild
	rm -rf container_composer_golangci_built.info
	rm -rf $(BUILDDIR)/$(PROCESSED_TEMPLATE_DIR)
	rm -rf $(BUILDDIR)/build/
	rm -f $(BUILDDIR)/go.local.*
	rm -f $(BUILDDIR)/container_worker_built.info
	rm -f $(BUILDDIR)/container_composer_built.info

.PHONY: push-check
push-check: lint build unit-tests srpm man
	./tools/check-runners
	./tools/check-snapshots --errors-only .
	rpmlint --config rpmlint.config $(CURDIR)/rpmbuild/SRPMS/*
	./tools/prepare-source.sh
	@if [ 0 -ne $$(git status --porcelain --untracked-files|wc -l) ]; then \
	    echo "There should be no changed or untracked files"; \
	    git status --porcelain --untracked-files; \
	    exit 1; \
	fi
	@echo "All looks good - congratulations"

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
.ONESHELL:
unit-tests:
	go test -race -covermode=atomic -coverprofile=coverage.txt -coverpkg=$$(go list ./... | tr "\n" ",") ./...
	# go modules with go.mod in subdirs are not tested automatically
	cd pkg/splunk_logger
	go test -race -covermode=atomic -coverprofile=../../coverage_splunk_logger.txt -coverpkg=$$(go list ./... | tr "\n" ",") ./...

.PHONY: coverage-report
coverage-report: unit-tests
	go tool cover -o coverage.html -html coverage.txt
	go tool cover -o coverage_splunk_logger.html -html coverage_splunk_logger.txt

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

.PHONY: $(RPM_SPECFILE)
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

container_composer_golangci_built.info: Makefile Containerfile_golangci_lint tools/apt-install-deps.sh
	podman build -f Containerfile_golangci_lint -t $(GOLANGCI_COMPOSER_IMAGE) --build-arg "GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION)"
	echo "Image last built on" > $@
	date >> $@

.PHONY: lint
lint: $(GOLANGCI_LINT_CACHE_DIR) container_composer_golangci_built.info
	podman run -t --rm -v $(SRCDIR):/app:z -v $(GOLANGCI_LINT_CACHE_DIR):/root/.cache:z -w /app $(GOLANGCI_COMPOSER_IMAGE) golangci-lint run -v

# The OpenShift CLI - maybe get it from https://access.redhat.com/downloads/content/290
OC_EXECUTABLE ?= oc

OPENSHIFT_TEMPLATES_DIR := templates/openshift
OPENSHIFT_TEMPLATES := $(notdir $(wildcard $(OPENSHIFT_TEMPLATES_DIR)/*.yml))

PROCESSED_TEMPLATE_DIR := $(BUILDDIR)/processed-templates

$(PROCESSED_TEMPLATE_DIR): $(BUILDDIR)
	mkdir -p $@

$(PROCESSED_TEMPLATE_DIR)/%.yml: $(PROCESSED_TEMPLATE_DIR) $(OPENSHIFT_TEMPLATES_DIR)/%.yml
	$(OC_EXECUTABLE) process -f $(OPENSHIFT_TEMPLATES_DIR)/$*.yml \
          -p IMAGE_TAG=image_tag \
          --local \
          -o yaml > $@

.PHONY: process-templates
process-templates: $(addprefix $(PROCESSED_TEMPLATE_DIR)/, $(OPENSHIFT_TEMPLATES))

# either "docker" or "sudo podman"
# podman needs to build as root as it also needs to run as root afterwards
CONTAINER_EXECUTABLE ?= sudo podman

DOCKER_IMAGE_WORKER := osbuild-worker_devel
DOCKERFILE_WORKER := distribution/Dockerfile-worker_srcinstall

DOCKER_IMAGE_COMPOSER := osbuild-composer_devel
DOCKERFILE_COMPOSER := distribution/Dockerfile-composer_srcinstall

GOPROXY ?= https://proxy.golang.org,direct

# source where the other repos are locally
# has to end with a trailing slash
SRC_DEPS_EXTERNAL_CHECKOUT_DIR ?= ../

# names of folder that have to be git-cloned additionally to be able
# to build all code
SRC_DEPS_EXTERNAL_NAMES := images pulp-client
SRC_DEPS_EXTERNAL_DIRS := $(addprefix $(SRC_DEPS_EXTERNAL_CHECKOUT_DIR),$(SRC_DEPS_EXTERNAL_NAMES))

$(SRC_DEPS_EXTERNAL_DIRS):
	@for DIR in $@; do if ! [ -d $$DIR ]; then echo "Please checkout $$DIR so it is available at $$DIR"; exit 1; fi; done


SRC_DEPS_DIRS := internal cmd pkg repositories

# All files to check for rebuild!
SRC_DEPS := $(shell find $(SRC_DEPS_DIRS) -name *.go -or -name *.json)
SRC_DEPS_EXTERNAL := $(shell find $(SRC_DEPS_EXTERNAL_DIRS) -name *.go)

# dependencies to rebuild worker
WORKER_SRC_DEPS := $(SRC_DEPS)
# dependencies to rebuild composer
COMPOSER_SRC_DEPS := $(SRC_DEPS)

GOMODARGS ?= -modfile=go.local.mod
# gcflags "-N -l" for golang to allow debugging
GCFLAGS ?= -gcflags=all=-N -gcflags=all=-l

CONTAINER_DEPS_COMPOSER := ./containers/osbuild-composer/entrypoint.py
CONTAINER_DEPS_WORKER := ./distribution/osbuild-worker-entrypoint.sh

USE_BTRFS ?= yes


# source where the other repos are locally
# has to end with a trailing slash
SRC_DEPS_EXTERNAL_CHECKOUT_DIR ?= ../

COMMON_SRC_DEPS_NAMES := osbuild
COMMON_SRC_DEPS_ORIGIN := $(addprefix $(SRC_DEPS_EXTERNAL_CHECKOUT_DIR),$(COMMON_SRC_DEPS_NAMES))

OSBUILD_CONTAINER_INDICATOR := $(SRC_DEPS_EXTERNAL_CHECKOUT_DIR)/osbuild/container_built.info

CONTAINER_EXECUTABLE ?= docker
MAKE_SUB_CALL := make CONTAINER_EXECUTABLE="$(CONTAINER_EXECUTABLE)"

$(COMMON_SRC_DEPS_ORIGIN):
	@for DIR in $@; do if ! [ -d $$DIR ]; then echo "Please checkout $$DIR so it is available at $$DIR"; exit 1; fi; done

# we'll trigger the sub-make for osbuild with "osbuild-container"
# and use OSBUILD_CONTAINER_INDICATOR to check if we need to rebuild our containers here
.PHONY: osbuild-container
$(OSBUILD_CONTAINER_INDICATOR) osbuild-container:
	$(MAKE_SUB_CALL) -C $(SRC_DEPS_EXTERNAL_CHECKOUT_DIR)osbuild container

go.local.mod go.local.sum: $(SRC_DEPS_EXTERNAL_DIRS) go.mod $(SRC_DEPS_EXTERNAL) $(WORKER_SRC_DEPS) $(COMPOSER_SRC_DEPS) Makefile
	cp go.mod go.local.mod
	cp go.sum go.local.sum

	go mod edit $(GOMODARGS) -replace github.com/osbuild/images=$(SRC_DEPS_EXTERNAL_CHECKOUT_DIR)images
	go mod edit $(GOMODARGS) -replace github.com/osbuild/pulp-client=$(SRC_DEPS_EXTERNAL_CHECKOUT_DIR)pulp-client
	go mod edit $(GOMODARGS) -replace github.com/osbuild/osbuild-composer/pkg/splunk_logger=./pkg/splunk_logger
	env GOPROXY=$(GOPROXY) go mod tidy $(GOMODARGS)
	env GOPROXY=$(GOPROXY) go mod vendor $(GOMODARGS)

container_worker_built.info: go.local.mod $(WORKER_SRC_DEPS) $(DOCKERFILE_WORKER) $(CONTAINER_DEPS_WORKER) $(OSBUILD_CONTAINER_INDICATOR)
	$(CONTAINER_EXECUTABLE) build -t $(DOCKER_IMAGE_WORKER) -f $(DOCKERFILE_WORKER) --build-arg GOMODARGS="$(GOMODARGS)"  --build-arg GCFLAGS="$(GCFLAGS)" --build-arg USE_BTRFS=$(USE_BTRFS) .
	echo "Worker last built on" > $@
	date >> $@

container_composer_built.info: go.local.mod $(COMPOSER_SRC_DEPS) $(DOCKERFILE_COMPOSER) $(CONTAINER_DEPS_COMPOSER) $(OSBUILD_CONTAINER_INDICATOR)
	$(CONTAINER_EXECUTABLE) build -t $(DOCKER_IMAGE_COMPOSER) -f $(DOCKERFILE_COMPOSER) --build-arg GOMODARGS="$(GOMODARGS)" --build-arg GCFLAGS="$(GCFLAGS)" .
	echo "Composer last built on" > $@
	date >> $@

# build a container with a worker from full source
.PHONY: container_worker
container_worker: osbuild-container container_worker_built.info

# build a container with the composer from full source
.PHONY: container_composer
container_composer: osbuild-container container_composer_built.info
