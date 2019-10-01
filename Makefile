build:
	go build -o osbuild-composer ./cmd/osbuild-composer/
	go build -o osbuild-worker ./cmd/osbuild-worker/

install:
	- mkdir -p /usr/lib/osbuild-composer
	cp osbuild-composer /usr/lib/osbuild-composer/
	cp osbuild-worker /usr/lib/osbuild-composer/
	cp dnf-json /usr/lib/osbuild-composer/

run-socket:
	systemd-socket-activate -l /run/weldr/api.socket -l /run/osbuild-composer/job.socket ./osbuild-composer
