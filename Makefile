.PHONY: build
build:
	go build -o osbuild-composer ./cmd/osbuild-composer/
	go build -o osbuild-worker ./cmd/osbuild-worker/
	go build -o osbuild-pipeline ./cmd/osbuild-pipeline/

.PHONY: install
install:
	- mkdir -p /usr/lib/osbuild-composer
	cp osbuild-composer /usr/lib/osbuild-composer/
	cp osbuild-worker /usr/lib/osbuild-composer/
	cp dnf-json /usr/lib/osbuild-composer/
	- mkdir -p /etc/sysusers.d/
	cp distribution/osbuild-composer.conf /etc/sysusers.d/
	systemd-sysusers osbuild-composer.conf
	- mkdir -p /etc/systemd/system/
	cp distribution/*.service /etc/systemd/system/
	cp distribution/*.socket /etc/systemd/system/
	systemctl daemon-reload
	systemctl enable osbuild-composer.socket
	systemctl enable osbuild-worker@1.service
