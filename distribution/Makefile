sd-install:
	cp *.service /etc/systemd/system/
	cp *.socket /etc/systemd/system/
	systemctl daemon-reload
	
sd-start:
	systemctl start osbuild-composer.socket
	systemctl start osbuild-worker.service

sd-enable:
	systemctl enable osbuild-composer.socket
	systemctl enable osbuild-worker.service

sd-stop:
	systemctl disable osbuild-composer.socket
	systemctl stop osbuild-composer.socket
	systemctl stop osbuild-composer.service
	systemctl disable osbuild-worker.service
	systemctl stop osbuild-worker.service

user-install:
	- mkdir -p /etc/sysusers.d/
	cp osbuild-composer.conf /etc/sysusers.d/
