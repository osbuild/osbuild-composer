PACKAGE_NAME = osbuild-composer
VERSION = $(shell grep Version golang-github-osbuild-composer.spec | awk '{gsub(/[^0-9]/,"")}1')

.PHONY: build
build:
	go build -o osbuild-composer ./cmd/osbuild-composer/
	go build -o osbuild-worker ./cmd/osbuild-worker/
	go build -o osbuild-pipeline ./cmd/osbuild-pipeline/
	go build -o osbuild-upload-azure ./cmd/osbuild-upload-azure/
	go build -o osbuild-upload-aws ./cmd/osbuild-upload-aws/

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

tarball:
	git archive --prefix=$(PACKAGE_NAME)-$(VERSION)/ --format=tar.gz HEAD > $(PACKAGE_NAME)-$(VERSION).tar.gz

srpm: golang-github-$(PACKAGE_NAME).spec check-working-directory tarball
	/usr/bin/rpmbuild -bs \
	  --define "_sourcedir $(CURDIR)" \
	  --define "_srcrpmdir $(CURDIR)" \
	  golang-github-$(PACKAGE_NAME).spec

rpm: golang-github-$(PACKAGE_NAME).spec check-working-directory tarball 
	- rm -r "`pwd`/output"
	mkdir -p "`pwd`/output"
	mkdir -p "`pwd`/rpmbuild"
	/usr/bin/rpmbuild -bb \
	  --define "_sourcedir `pwd`" \
	  --define "_specdir `pwd`" \
	  --define "_builddir `pwd`/rpmbuild" \
	  --define "_srcrpmdir `pwd`" \
	  --define "_rpmdir `pwd`/output" \
	  --define "_buildrootdir `pwd`/build" \
	  golang-github-$(PACKAGE_NAME).spec
	rm -r "`pwd`/rpmbuild"
	rm -r "`pwd`/build"

check-working-directory:
	@if [ "`git status --porcelain --untracked-files=no | wc -l`" != "0" ]; then \
	  echo "Uncommited changes, refusing (Use git add . && git commit or git stash to clean your working directory)."; \
	  exit 1; \
	fi
