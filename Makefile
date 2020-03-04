PACKAGE_NAME = osbuild-composer
COMMIT=$(shell git rev-parse HEAD)

.PHONY: build
build:
	go build -o osbuild-composer ./cmd/osbuild-composer/
	go build -o osbuild-worker ./cmd/osbuild-worker/
	go build -o osbuild-pipeline ./cmd/osbuild-pipeline/
	go build -o osbuild-upload-azure ./cmd/osbuild-upload-azure/
	go build -o osbuild-upload-aws ./cmd/osbuild-upload-aws/
	go build -o osbuild-tests ./cmd/osbuild-tests/
	go build -o osbuild-weldr-tests ./cmd/osbuild-weldr-tests/
	go build -o osbuild-dnf-json-tests ./cmd/osbuild-dnf-json-tests/

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

.PHONY: rpm
rpm:
	mkdir -p $(CURDIR)/rpmbuild/SPECS $(CURDIR)/rpmbuild/SOURCES

	echo "%global commit $(COMMIT)" | cat - golang-github-osbuild-composer.spec \
		> rpmbuild/SPECS/golang-github-osbuild-composer-$(COMMIT).spec

	git archive --prefix=$(PACKAGE_NAME)-$(COMMIT)/ --format=tar.gz HEAD \
		> rpmbuild/SOURCES/$(PACKAGE_NAME)-$(COMMIT).tar.gz

	rpmbuild -bs \
		--define "_topdir $(CURDIR)/rpmbuild" \
		rpmbuild/SPECS/golang-github-$(PACKAGE_NAME)-$(COMMIT).spec

	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		rpmbuild/SPECS/golang-github-$(PACKAGE_NAME)-$(COMMIT).spec
