# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.vm.box = "fedora/31-cloud-base"
  config.vm.network "forwarded_port", guest: 9090, host: 9091

  # This is needed for dnf, without all the RAM OOM killer will kill it during
  # depsolving :)
  config.vm.provider :libvirt do |libvirt|
    libvirt.memory = 2048
    libvirt.cpus = 2
  end

  config.vm.provision "shell", inline: <<-SHELL
    set -e

    # add admin account with foobar password to be able to log in to cockpit
    # (inspired by cockpit repository)
    getent passwd admin >/dev/null || useradd -c Administrator -G wheel admin
    echo foobar | passwd --stdin admin

    dnf upgrade -y
    dnf install -y cockpit-composer /vagrant/.build/*.rpm

    systemctl enable --now cockpit.socket
  SHELL
end
