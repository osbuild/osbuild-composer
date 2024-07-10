# Do not build with tests by default
# Pass --with tests to rpmbuild to override
%bcond_with tests

# When --with relax_requires is specified osbuild-composer-tests
# will require osbuild-composer only by name, excluding version/release
# This is used internally during nightly pipeline testing!
%bcond_with relax_requires

# The minimum required osbuild version
%global min_osbuild_version 116

%global goipath         github.com/osbuild/osbuild-composer

Version:        113

%gometa

%global common_description %{expand:
A service for building customized OS artifacts, such as VM images and OSTree
commits, that uses osbuild under the hood. Besides building images for local
usage, it can also upload images directly to cloud.

It is compatible with composer-cli and cockpit-composer clients.
}

Name:           osbuild-composer
Release:        1%{?dist}
Summary:        An image building service based on osbuild

# osbuild-composer doesn't have support for building i686 and armv7hl images
ExcludeArch:    i686 armv7hl

# Upstream license specification: Apache-2.0
License:        Apache-2.0
URL:            %{gourl}
Source0:        %{gosource}


BuildRequires:  %{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}
BuildRequires:  systemd
BuildRequires:  krb5-devel
BuildRequires:  python3-docutils
BuildRequires:  make
# Build requirements of 'theproglottis/gpgme' package
BuildRequires:  gpgme-devel
BuildRequires:  libassuan-devel
# Build requirements of 'github.com/containers/storage' package
BuildRequires:  device-mapper-devel
%if 0%{?fedora}
BuildRequires:  systemd-rpm-macros
BuildRequires:  git
# Build requirements of 'github.com/containers/storage' package
BuildRequires:  btrfs-progs-devel
# DO NOT REMOVE the BUNDLE_START and BUNDLE_END markers as they are used by 'tools/rpm_spec_add_provides_bundle.sh' to generate the Provides: bundled list
# BUNDLE_START
# BUNDLE_END
%endif

Requires: %{name}-core = %{version}-%{release}
Requires: %{name}-worker = %{version}-%{release}
Requires: systemd

Provides: weldr

%description
%{common_description}

%prep
%if 0%{?rhel}
%forgeautosetup -p1
%else
%goprep -k
%endif

%build
export GOFLAGS="-buildmode=pie"
%if 0%{?rhel}
GO_BUILD_PATH=$PWD/_build
install -m 0755 -vd $(dirname $GO_BUILD_PATH/src/%{goipath})
ln -fs $PWD $GO_BUILD_PATH/src/%{goipath}
cd $GO_BUILD_PATH/src/%{goipath}
install -m 0755 -vd _bin
export PATH=$PWD/_bin${PATH:+:$PATH}
export GOPATH=$GO_BUILD_PATH:%{gopath}
export GOFLAGS+=" -mod=vendor"
%endif
%if 0%{?fedora}
# Fedora disables Go modules by default, but we want to use them.
# Undefine the macro which disables it to use the default behavior.
%undefine gomodulesmode
%endif

# btrfs-progs-devel is not available on RHEL
%if 0%{?rhel}
GOTAGS="exclude_graphdriver_btrfs"
%endif

# Set the commit hash so that composer can report what source version
# was used to build it. This has to be set explicitly when calling rpmbuild,
# this script will not attempt to automatically discover it.
%if %{?commit:1}0
export LDFLAGS="${LDFLAGS} -X 'github.com/osbuild/osbuild-composer/internal/common.GitRev=%{commit}'"
%endif
export LDFLAGS="${LDFLAGS} -X 'github.com/osbuild/osbuild-composer/internal/common.RpmVersion=%{name}-%{?epoch:%epoch:}%{version}-%{release}.%{_arch}'"

%gobuild ${GOTAGS:+-tags=$GOTAGS} -o _bin/osbuild-composer %{goipath}/cmd/osbuild-composer
%gobuild ${GOTAGS:+-tags=$GOTAGS} -o _bin/osbuild-worker %{goipath}/cmd/osbuild-worker
%gobuild ${GOTAGS:+-tags=$GOTAGS} -o _bin/osbuild-worker-executor %{goipath}/cmd/osbuild-worker-executor
%gobuild ${GOTAGS:+-tags=$GOTAGS} -o _bin/osbuild-jobsite-manager %{goipath}/cmd/osbuild-jobsite-manager
%gobuild ${GOTAGS:+-tags=$GOTAGS} -o _bin/osbuild-jobsite-builder %{goipath}/cmd/osbuild-jobsite-builder

make man

%if %{with tests} || 0%{?rhel}

# Build test binaries with `go test -c`, so that they can take advantage of
# golang's testing package. The golang rpm macros don't support building them
# directly. Thus, do it manually, taking care to also include a build id.
#
# On Fedora, also turn off go modules and set the path to the one into which
# the golang-* packages install source code.
%if 0%{?fedora}
export GO111MODULE=off
export GOPATH=%{gobuilddir}:%{gopath}
%endif

TEST_LDFLAGS="${LDFLAGS:-} -B 0x$(od -N 20 -An -tx1 -w100 /dev/urandom | tr -d ' ')"

%if 0%{?rhel}
GOTAGS="${GOTAGS:+$GOTAGS,}rhel%{rhel}"
%endif

go test -c -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-composer-cli-tests %{goipath}/cmd/osbuild-composer-cli-tests
go test -c -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-dnf-json-tests %{goipath}/cmd/osbuild-dnf-json-tests
go test -c -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-weldr-tests %{goipath}/internal/client/
go test -c -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-image-tests %{goipath}/cmd/osbuild-image-tests
go test -c -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-auth-tests %{goipath}/cmd/osbuild-auth-tests
go test -c -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-koji-tests %{goipath}/cmd/osbuild-koji-tests
go test -c -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-composer-dbjobqueue-tests %{goipath}/cmd/osbuild-composer-dbjobqueue-tests
go test -c -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-service-maintenance-tests %{goipath}/cmd/osbuild-service-maintenance
go build -tags="integration${GOTAGS:+,$GOTAGS}" -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-mock-openid-provider %{goipath}/cmd/osbuild-mock-openid-provider

%endif

%install
install -m 0755 -vd                                                %{buildroot}%{_libexecdir}/osbuild-composer
install -m 0755 -vp _bin/osbuild-composer                          %{buildroot}%{_libexecdir}/osbuild-composer/
install -m 0755 -vp _bin/osbuild-worker                            %{buildroot}%{_libexecdir}/osbuild-composer/
install -m 0755 -vp _bin/osbuild-worker-executor                   %{buildroot}%{_libexecdir}/osbuild-composer/
install -m 0755 -vp _bin/osbuild-jobsite-manager                   %{buildroot}%{_libexecdir}/osbuild-composer/
install -m 0755 -vp _bin/osbuild-jobsite-builder                   %{buildroot}%{_libexecdir}/osbuild-composer/

# Only include repositories for the distribution and release
install -m 0755 -vd                                                %{buildroot}%{_datadir}/osbuild-composer/repositories

# CentOS also defines rhel so we check for centos first
%if 0%{?centos}

# Latest CentOS supports building all CentOS versions
%if 0%{?centos} >= 10
install -m 0644 -vp repositories/centos-*                          %{buildroot}%{_datadir}/osbuild-composer/repositories/

%else
# All other CentOS versions support building for the same version
install -m 0644 -vp repositories/centos-%{centos}*                 %{buildroot}%{_datadir}/osbuild-composer/repositories/
install -m 0644 -vp repositories/centos-stream-%{centos}*          %{buildroot}%{_datadir}/osbuild-composer/repositories/
%endif

%else

%if 0%{?rhel}
# RHEL 10 supports building all RHEL versions
%if 0%{?rhel} >= 10
install -m 0644 -vp repositories/rhel-*                            %{buildroot}%{_datadir}/osbuild-composer/repositories/

# RHEL-8 auxiliary key is signed using SHA-1, which is not enabled by default on RHEL-10 and later
for REPO_FILE in $(ls repositories/rhel-8*-no-aux-key.json); do
    install -m 0644 -vp ${REPO_FILE}                               %{buildroot}%{_datadir}/osbuild-composer/repositories/$(basename ${REPO_FILE} | sed 's/-no-aux-key//g')
done

%else
# All other RHEL versions support building for the same version
install -m 0644 -vp repositories/rhel-%{rhel}*                     %{buildroot}%{_datadir}/osbuild-composer/repositories/

# RHEL 9 supports building also for RHEL 8
%if 0%{?rhel} == 9
install -m 0644 -vp repositories/rhel-8*                           %{buildroot}%{_datadir}/osbuild-composer/repositories/
%endif

%endif
%endif
%endif

# Fedora can build for all included fedora releases
%if 0%{?fedora}
install -m 0644 -vp repositories/fedora-*                          %{buildroot}%{_datadir}/osbuild-composer/repositories/
%endif

install -m 0755 -vd                                                %{buildroot}%{_unitdir}
install -m 0644 -vp distribution/*.{service,socket}                %{buildroot}%{_unitdir}/

install -m 0755 -vd                                                %{buildroot}%{_sysusersdir}
install -m 0644 -vp distribution/osbuild-composer.conf             %{buildroot}%{_sysusersdir}/

install -m 0755 -vd                                                %{buildroot}%{_localstatedir}/cache/osbuild-composer/dnf-cache

install -m 0755 -vd                                                %{buildroot}%{_mandir}/man7
install -m 0644 -vp docs/*.7                                       %{buildroot}%{_mandir}/man7/

%if %{with tests} || 0%{?rhel}

install -m 0755 -vd                                                %{buildroot}%{_libexecdir}/osbuild-composer-test
install -m 0755 -vp _bin/osbuild-composer-cli-tests                %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp _bin/osbuild-weldr-tests                       %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp _bin/osbuild-dnf-json-tests                    %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp _bin/osbuild-image-tests                       %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp _bin/osbuild-auth-tests                        %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp _bin/osbuild-koji-tests                        %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp _bin/osbuild-composer-dbjobqueue-tests         %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp _bin/osbuild-service-maintenance-tests         %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp _bin/osbuild-mock-openid-provider              %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/define-compose-url.sh                    %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/provision.sh                             %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/gen-certs.sh                             %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/gen-ssh.sh                               %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/image-info                               %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/run-koji-container.sh                    %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/koji-compose.py                          %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/libvirt_test.sh                          %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/s3_test.sh                               %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/generic_s3_test.sh                       %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/generic_s3_https_test.sh                 %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/run-mock-auth-servers.sh                 %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vp tools/set-env-variables.sh                     %{buildroot}%{_libexecdir}/osbuild-composer-test/
install -m 0755 -vd                                                %{buildroot}%{_libexecdir}/tests/osbuild-composer
install -m 0755 -vp test/cases/*.sh                                %{buildroot}%{_libexecdir}/tests/osbuild-composer/

install -m 0755 -vd                                                %{buildroot}%{_libexecdir}/tests/osbuild-composer/api
install -m 0755 -vp test/cases/api/*.sh                            %{buildroot}%{_libexecdir}/tests/osbuild-composer/api/

install -m 0755 -vd                                                %{buildroot}%{_libexecdir}/tests/osbuild-composer/api/common
install -m 0755 -vp test/cases/api/common/*.sh                     %{buildroot}%{_libexecdir}/tests/osbuild-composer/api/common/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/ansible
install -m 0644 -vp test/data/ansible/*                            %{buildroot}%{_datadir}/tests/osbuild-composer/ansible/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/azure
install -m 0644 -vp test/data/azure/*                              %{buildroot}%{_datadir}/tests/osbuild-composer/azure/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/manifests
install -m 0644 -vp test/data/manifests/*                          %{buildroot}%{_datadir}/tests/osbuild-composer/manifests/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/cloud-init
install -m 0644 -vp test/data/cloud-init/*                         %{buildroot}%{_datadir}/tests/osbuild-composer/cloud-init/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/composer
install -m 0644 -vp test/data/composer/*                           %{buildroot}%{_datadir}/tests/osbuild-composer/composer/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/worker
install -m 0644 -vp test/data/worker/*                             %{buildroot}%{_datadir}/tests/osbuild-composer/worker/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/repositories
install -m 0644 -vp test/data/repositories/*                       %{buildroot}%{_datadir}/tests/osbuild-composer/repositories/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/kerberos
install -m 0644 -vp test/data/kerberos/*                           %{buildroot}%{_datadir}/tests/osbuild-composer/kerberos/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/keyring
install -m 0644 -vp test/data/keyring/id_rsa.pub                   %{buildroot}%{_datadir}/tests/osbuild-composer/keyring/
install -m 0600 -vp test/data/keyring/id_rsa                       %{buildroot}%{_datadir}/tests/osbuild-composer/keyring/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/koji
install -m 0644 -vp test/data/koji/*                               %{buildroot}%{_datadir}/tests/osbuild-composer/koji/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/x509
install -m 0644 -vp test/data/x509/*                               %{buildroot}%{_datadir}/tests/osbuild-composer/x509/

install -m 0755 -vd                                                %{buildroot}%{_datadir}/tests/osbuild-composer/schemas
install -m 0644 -vp pkg/jobqueue/dbjobqueue/schemas/*              %{buildroot}%{_datadir}/tests/osbuild-composer/schemas/

install -m 0755 -vd                                               %{buildroot}%{_datadir}/tests/osbuild-composer/upgrade8to9
install -m 0644 -vp test/data/upgrade8to9/*                       %{buildroot}%{_datadir}/tests/osbuild-composer/upgrade8to9/

%endif

%check
export GOFLAGS="-buildmode=pie"
%if 0%{?rhel}
export GOFLAGS+=" -mod=vendor -tags=exclude_graphdriver_btrfs"
export GOPATH=$PWD/_build:%{gopath}
# cd inside GOPATH, otherwise go with GO111MODULE=off ignores vendor directory
cd $PWD/_build/src/%{goipath}
%gotest ./...
%else
%gocheck
%endif

%post
%systemd_post osbuild-composer.service osbuild-composer.socket osbuild-composer-api.socket osbuild-composer-prometheus.socket osbuild-remote-worker.socket

%preun
%systemd_preun osbuild-composer.service osbuild-composer.socket osbuild-composer-api.socket osbuild-composer-prometheus.socket osbuild-remote-worker.socket

%postun
%systemd_postun_with_restart osbuild-composer.service osbuild-composer.socket osbuild-composer-api.socket osbuild-composer-prometheus.socket osbuild-remote-worker.socket

%files
%license LICENSE
%doc README.md
%{_mandir}/man7/%{name}.7*
%{_unitdir}/osbuild-composer.service
%{_unitdir}/osbuild-composer.socket
%{_unitdir}/osbuild-composer-api.socket
%{_unitdir}/osbuild-composer-prometheus.socket
%{_unitdir}/osbuild-local-worker.socket
%{_unitdir}/osbuild-remote-worker.socket
%{_sysusersdir}/osbuild-composer.conf

%package core
Summary:    The core osbuild-composer binary
Requires:   osbuild-depsolve-dnf >= %{min_osbuild_version}
Provides:   %{name}-dnf-json = %{version}-%{release}
Obsoletes:  %{name}-dnf-json < %{version}-%{release}

%description core
The core osbuild-composer binary. This is suitable both for spawning in containers and by systemd.

%files core
%{_libexecdir}/osbuild-composer/osbuild-composer
%{_datadir}/osbuild-composer/

%package worker
Summary:    The worker for osbuild-composer
Requires:   systemd
Requires:   qemu-img
Requires:   osbuild >= %{min_osbuild_version}
Requires:   osbuild-ostree >= %{min_osbuild_version}
Requires:   osbuild-lvm2 >= %{min_osbuild_version}
Requires:   osbuild-luks2 >= %{min_osbuild_version}
Requires:   osbuild-depsolve-dnf >= %{min_osbuild_version}
Provides:   %{name}-dnf-json = %{version}-%{release}
Obsoletes:  %{name}-dnf-json < %{version}-%{release}

%description worker
The worker for osbuild-composer

%files worker
%{_libexecdir}/osbuild-composer/osbuild-worker
%{_libexecdir}/osbuild-composer/osbuild-worker-executor
%{_libexecdir}/osbuild-composer/osbuild-jobsite-manager
%{_libexecdir}/osbuild-composer/osbuild-jobsite-builder
%{_unitdir}/osbuild-worker@.service
%{_unitdir}/osbuild-remote-worker@.service

%post worker
%systemd_post osbuild-worker@.service osbuild-remote-worker@.service

%preun worker
# systemd_preun uses systemctl disable --now which doesn't work well with template services.
# See https://github.com/systemd/systemd/issues/15620
# The following lines mimicks its behaviour by running two commands.
# The scriptlet is supposed to run only when the package is being removed.
if [ $1 -eq 0 ] && [ -d /run/systemd/system ]; then
    # disable and stop all the worker services
    systemctl --no-reload disable osbuild-worker@.service osbuild-remote-worker@.service
    systemctl stop "osbuild-worker@*.service" "osbuild-remote-worker@*.service"
fi

%postun worker
# restart all the worker services
%systemd_postun_with_restart "osbuild-worker@*.service" "osbuild-remote-worker@*.service"

%if %{with tests} || 0%{?rhel}

%package tests
Summary:    Integration tests
%if %{with relax_requires}
Requires:   %{name}
%else
Requires:   %{name} = %{version}-%{release}
%endif
Requires:   composer-cli
Requires:   createrepo_c
Requires:   xorriso
Requires:   qemu-kvm-core
Requires:   systemd-container
Requires:   jq
Requires:   unzip
Requires:   container-selinux
Requires:   dnsmasq
Requires:   krb5-workstation
Requires:   podman
Requires:   python3
Requires:   sssd-krb5
Requires:   libvirt-client libvirt-daemon
Requires:   libvirt-daemon-config-network
Requires:   libvirt-daemon-config-nwfilter
Requires:   libvirt-daemon-driver-interface
Requires:   libvirt-daemon-driver-network
Requires:   libvirt-daemon-driver-nodedev
Requires:   libvirt-daemon-driver-nwfilter
Requires:   libvirt-daemon-driver-qemu
Requires:   libvirt-daemon-driver-secret
Requires:   libvirt-daemon-driver-storage
Requires:   libvirt-daemon-driver-storage-disk
Requires:   libvirt-daemon-kvm
Requires:   qemu-img
Requires:   qemu-kvm
Requires:   rpmdevtools
Requires:   virt-install
Requires:   expect
Requires:   python3-lxml
Requires:   httpd
Requires:   mod_ssl
Requires:   openssl
Requires:   firewalld
# podman-plugins has been deprecated since podman version 5.0.0,
# which is in Fedora 40+ and in c10s / el10
%if (0%{?rhel} && 0%{?rhel} < 10) || (0%{?fedora} && 0%{?fedora} < 40)
Requires:   podman-plugins
%endif
Requires:   dnf-plugins-core
Requires:   skopeo
Requires:   make
Requires:   python3-pip
%if 0%{?fedora}
# koji and ansible are not in RHEL repositories. Depending on them breaks RHEL
# gating (see OSCI-1541). The test script must enable EPEL and install those
# packages manually.
Requires:   koji
Requires:   ansible
%endif
%ifarch %{arm}
Requires:   edk2-aarch64
%endif

%description tests
Integration tests to be run on a pristine-dedicated system to test the osbuild-composer package.

%files tests
%{_libexecdir}/osbuild-composer-test/
%{_libexecdir}/tests/osbuild-composer/
%{_datadir}/tests/osbuild-composer/

%endif

%changelog
# the changelog is distribution-specific, therefore there's just one entry
# to make rpmlint happy.

* Wed Sep 11 2019 Image Builder team <osbuilders@redhat.com> - 0-1
- On this day, this project was born.
