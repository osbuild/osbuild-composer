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

.ONESHELL:
SHELL := /bin/bash
.SHELLFLAGS := -ec -o pipefail

# see https://hub.docker.com/r/golangci/golangci-lint/tags
# This is also used in Containerfile_golangci_lint FROM line
# v1.60 to get golang 1.23 (1.23.0)
# v1.56 to get golang 1.22 (1.22.0)
# v1.55 to get golang 1.21 (1.21.3)
# v1.53 to get golang 1.20 (1.20.5)
GOLANGCI_LINT_VERSION=v1.61
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
	@echo "    db-tests:           Run postgres DB tests"
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

.PHONY: build-maintenance
build-maintenance: $(BUILDDIR)/bin/
	go build -o $<osbuild-service-maintenance ./cmd/osbuild-service-maintenance
	go test -c -tags=integration -o $<osbuild-composer-maintenance-tests ./cmd/osbuild-service-maintenance/

.PHONY: build
build: $(BUILDDIR)/bin/ build-maintenance
	go build -o $<osbuild-composer ./cmd/osbuild-composer/
	go build -o $<osbuild-worker ./cmd/osbuild-worker/
	go build -o $<osbuild-worker-executor ./cmd/osbuild-worker-executor/
	go build -o $<osbuild-upload-azure ./cmd/osbuild-upload-azure/
	go build -o $<osbuild-upload-aws ./cmd/osbuild-upload-aws/
	go build -o $<osbuild-upload-gcp ./cmd/osbuild-upload-gcp/
	go build -o $<osbuild-upload-oci ./cmd/osbuild-upload-oci/
	go build -o $<osbuild-upload-generic-s3 ./cmd/osbuild-upload-generic-s3/
	go build -o $<osbuild-mock-openid-provider ./cmd/osbuild-mock-openid-provider
	# also build the test binaries
	go test -c -tags=integration -o $<osbuild-composer-cli-tests ./cmd/osbuild-composer-cli-tests/main_test.go
	go test -c -tags=integration -o $<osbuild-weldr-tests ./internal/client/
	go test -c -tags=integration -o $<osbuild-auth-tests ./cmd/osbuild-auth-tests/
	go test -c -tags=integration -o $<osbuild-koji-tests ./cmd/osbuild-koji-tests/
	go test -c -tags=integration -o $<osbuild-composer-dbjobqueue-tests ./cmd/osbuild-composer-dbjobqueue-tests/

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
clean: db-tests-prune
	rm -rf $(BUILDDIR)/bin/
	rm -rf $(CURDIR)/rpmbuild
	rm -rf container_composer_golangci_built.info
	rm -rf $(BUILDDIR)/$(PROCESSED_TEMPLATE_DIR)
	rm -rf $(GOLANGCI_LINT_CACHE_DIR)

.PHONY: push-check
push-check: lint build unit-tests srpm man
	./tools/check-runners
	./tools/check-snapshots --errors-only .
	rpmlint --config rpmlint.config $(CURDIR)/rpmbuild/SRPMS/*
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
unit-tests:
	go test -race -covermode=atomic -coverprofile=coverage.txt -coverpkg=$$(go list ./... | tr "\n" ",") ./...
	# go modules with go.mod in subdirs are not tested automatically
	cd pkg/splunk_logger
	go test -race -covermode=atomic -coverprofile=../../coverage_splunk_logger.txt -coverpkg=$$(go list ./... | tr "\n" ",") ./...

.PHONY: coverage-report
coverage-report: unit-tests
	go tool cover -o coverage.html -html coverage.txt
	go tool cover -o coverage_splunk_logger.html -html coverage_splunk_logger.txt

CONTAINER_EXECUTABLE ?= podman

.PHONY: db-tests-prune
db-tests-prune:
	-$(CONTAINER_EXECUTABLE) stop composer-test-db
	-$(CONTAINER_EXECUTABLE) rm composer-test-db

CHECK_DB_PORT_READY=$(CONTAINER_EXECUTABLE) exec composer-test-db pg_isready -d osbuildcomposer
CHECK_DB_UP=$(CONTAINER_EXECUTABLE) exec composer-test-db psql -U postgres -d osbuildcomposer -c "SELECT 1"

.PHONY: db-tests
db-tests:
	-$(CONTAINER_EXECUTABLE) stop composer-test-db 2>/dev/null || echo "DB already stopped"
	-$(CONTAINER_EXECUTABLE) rm composer-test-db 2>/dev/null || echo "DB already removed"
	$(CONTAINER_EXECUTABLE) run -d \
      --name composer-test-db \
      --env POSTGRES_PASSWORD=foobar \
      --env POSTGRES_DB=osbuildcomposer \
      --publish 5432:5432 \
      postgres:12
	echo "Waiting for DB"
	until $(CHECK_DB_PORT_READY) ; do sleep 1; done
	until $(CHECK_DB_UP) ; do sleep 1; done
	env PGPASSWORD=foobar \
	    PGDATABASE=osbuildcomposer \
	    PGUSER=postgres \
	    PGHOST=localhost \
	    PGPORT=5432 \
	    ./tools/dbtest-run-migrations.sh
	./tools/dbtest-entrypoint.sh
	# we'll leave the composer-test-db container running
	# for easier inspection is something fails

.PHONY: test
test: unit-tests db-tests  # run all tests

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

# trying to catch our use cases of the github action implementation
# https://github.com/ludeeus/action-shellcheck/blob/master/action.yaml#L164
SHELLCHECK_FILES=$(shell find . -name "*.sh" -not -regex "./vendor/.*")

.PHONY: lint
lint: $(GOLANGCI_LINT_CACHE_DIR) container_composer_golangci_built.info
	./tools/prepare-source.sh
	podman run -t --rm -v $(SRCDIR):/app:z -v $(GOLANGCI_LINT_CACHE_DIR):/root/.cache:z -w /app $(GOLANGCI_COMPOSER_IMAGE) golangci-lint config verify -v
	podman run -t --rm -v $(SRCDIR):/app:z -v $(GOLANGCI_LINT_CACHE_DIR):/root/.cache:z -w /app $(GOLANGCI_COMPOSER_IMAGE) golangci-lint run -v
	echo "$(SHELLCHECK_FILES)" | xargs shellcheck --shell bash -e SC1091 -e SC2002 -e SC2317

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

# get yourself aws access to your deployment by
# either setting AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
# or providing a token in ~/.aws/credentials
# for profile "default"!
.PHONY: osbuild-service-maintenance-dry-run-aws
osbuild-service-maintenance-dry-run-aws: build-maintenance
	env DRY_RUN=true \
	    ENABLE_AWS_MAINTENANCE=true \
	    $(BUILDDIR)/bin/osbuild-service-maintenance
