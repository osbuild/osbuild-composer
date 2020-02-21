# -*- mode: ruby -*-
# vi: set ft=ruby :

# You can use this file to easily run integration test. In order to run them,
# build RPM using `make rpm` and then run `vagrant up`. If all went just fine,
# you can run `vagrant destroy` to clean up the VM.
#
# If anything went wrong, you can `vagrant ssh` into the machine and try to
# tweak the tests by hand there, or you can fix them locally and run `vagrant
# rsync-auto` to sync new RPMs, then ssh into the machine, 
# `dnf remove 'golang-github-osbuild-composer-*'`, log out of the machine, and
# run `vagrant provision` (yes, it was not intended for this use case, but works
# just fine).

Vagrant.configure("2") do |config|
  config.vm.box = "fedora/31-cloud-base"

  # This is needed for dnf, without all the RAM OOM killer will kill it during
  # depsolving :)
  config.vm.provider :libvirt do |libvirt|
    libvirt.memory = 4096
    libvirt.cpus = 2
  end

  # :-O what the sed?!
  config.vm.provision "shell", inline: <<-SHELL
    dnf install /vagrant/output/x86_64/*.rpm -y && \
    systemctl start osbuild-composer.socket && \
    pushd /usr/libexec/osbuild-composer && \
    su vagrant -c /usr/libexec/tests/osbuild-composer/osbuild-tests
  SHELL
end
