%global goipath         github.com/osbuild/osbuild-composer

Version:        7

%gometa

%global common_description %{expand:
An image building service based on osbuild
It is inspired by lorax-composer and exposes the same API.
As such, it is a drop-in replacement.
}

Name:           %{goname}
Release:        1%{?dist}
Summary:        An image building service based on osbuild

# Upstream license specification: Apache-2.0
License:        ASL 2.0
URL:            %{gourl}
Source0:        %{gosource}


BuildRequires:  %{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}
BuildRequires:  systemd
%if 0%{?fedora}
BuildRequires:  systemd-rpm-macros
BuildRequires:  git
BuildRequires:  golang(github.com/aws/aws-sdk-go)
BuildRequires:  golang-github-azure-storage-blob-devel
BuildRequires:  golang(github.com/BurntSushi/toml)
BuildRequires:  golang(github.com/coreos/go-semver/semver)
BuildRequires:  golang(github.com/coreos/go-systemd/activation)
BuildRequires:  golang(github.com/google/uuid)
BuildRequires:  golang(github.com/julienschmidt/httprouter)
BuildRequires:  golang(github.com/gobwas/glob)
BuildRequires:  golang(github.com/google/go-cmp/cmp)
%endif

Requires: golang-github-osbuild-composer-worker
Requires: systemd
Requires: osbuild >= 7

Provides: osbuild-composer
Provides: weldr

%description
%{common_description}

%prep
%if 0%{?rhel}
%forgeautosetup -p1
%else
%goprep
%endif

%build
%if 0%{?rhel}
GO_BUILD_PATH=$PWD/_build
install -m 0755 -vd $(dirname $GO_BUILD_PATH/src/%{goipath})
ln -fs $PWD $GO_BUILD_PATH/src/%{goipath}
cd $GO_BUILD_PATH/src/%{goipath}
install -m 0755 -vd _bin
export PATH=$PWD/_bin${PATH:+:$PATH}
export GOPATH=$GO_BUILD_PATH:%{gopath}
export GOFLAGS=-mod=vendor
%endif
%gobuild -o _bin/osbuild-composer %{goipath}/cmd/osbuild-composer
%gobuild -o _bin/osbuild-worker %{goipath}/cmd/osbuild-worker
%gobuild -o _bin/osbuild-image-tests %{goipath}/cmd/osbuild-image-tests
%gobuild -o _bin/osbuild-rcm-tests %{goipath}/cmd/osbuild-rcm-tests

# Build test binaries with `go test -c`, so that they can take advantage of
# golang's testing package. The golang rpm macros don't support building them
# directly. Thus, do it manually, taking care to also include a build id.

TEST_LDFLAGS="${LDFLAGS:-} -B 0x$(od -N 20 -An -tx1 -w100 /dev/urandom | tr -d ' ')"

go test -c -tags=integration -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-tests %{goipath}/cmd/osbuild-tests
go test -c -tags=integration -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-dnf-json-tests %{goipath}/cmd/osbuild-dnf-json-tests
go test -c -tags=integration -ldflags="${TEST_LDFLAGS}" -o _bin/osbuild-weldr-tests %{goipath}/internal/weldrcheck/

%install
install -m 0755 -vd                                         %{buildroot}%{_libexecdir}/osbuild-composer
install -m 0755 -vp _bin/osbuild-composer                   %{buildroot}%{_libexecdir}/osbuild-composer/
install -m 0755 -vp _bin/osbuild-worker                     %{buildroot}%{_libexecdir}/osbuild-composer/
install -m 0755 -vp dnf-json                                %{buildroot}%{_libexecdir}/osbuild-composer/

install -m 0755 -vd                                         %{buildroot}%{_libexecdir}/tests/osbuild-composer
install -m 0755 -vp _bin/osbuild-tests                      %{buildroot}%{_libexecdir}/tests/osbuild-composer/
install -m 0755 -vp _bin/osbuild-weldr-tests                %{buildroot}%{_libexecdir}/tests/osbuild-composer/
install -m 0755 -vp _bin/osbuild-dnf-json-tests             %{buildroot}%{_libexecdir}/tests/osbuild-composer/
install -m 0755 -vp _bin/osbuild-image-tests                %{buildroot}%{_libexecdir}/tests/osbuild-composer/
install -m 0755 -vp _bin/osbuild-rcm-tests                  %{buildroot}%{_libexecdir}/tests/osbuild-composer/
install -m 0755 -vp tools/image-info                        %{buildroot}%{_libexecdir}/osbuild-composer/

install -m 0755 -vd                                         %{buildroot}%{_datadir}/osbuild-composer/repositories
install -m 0644 -vp repositories/*                          %{buildroot}%{_datadir}/osbuild-composer/repositories/

install -m 0755 -vd                                         %{buildroot}%{_datadir}/tests/osbuild-composer/cases
install -m 0644 -vp test/cases/*                            %{buildroot}%{_datadir}/tests/osbuild-composer/cases/
install -m 0755 -vd                                         %{buildroot}%{_datadir}/tests/osbuild-composer/keyring
install -m 0600 -vp test/keyring/*                          %{buildroot}%{_datadir}/tests/osbuild-composer/keyring/

install -m 0755 -vd                                         %{buildroot}%{_datadir}/tests/osbuild-composer/cloud-init
install -m 0644 -vp test/cloud-init/*                       %{buildroot}%{_datadir}/tests/osbuild-composer/cloud-init/

install -m 0755 -vd                                         %{buildroot}%{_unitdir}
install -m 0644 -vp distribution/*.{service,socket}         %{buildroot}%{_unitdir}/

install -m 0755 -vd                                         %{buildroot}%{_sysusersdir}
install -m 0644 -vp distribution/osbuild-composer.conf      %{buildroot}%{_sysusersdir}/

install -m 0755 -vd                                         %{buildroot}%{_localstatedir}/cache/osbuild-composer/dnf-cache

%check
%if 0%{?rhel}
export GOFLAGS=-mod=vendor
export GOPATH=$PWD/_build:%{gopath}
%gotest ./...
%else
%gocheck
%endif

%post
%systemd_post osbuild-composer.service osbuild-composer.socket osbuild-remote-worker.socket

%preun
%systemd_preun osbuild-composer.service osbuild-composer.socket osbuild-remote-worker.socket

%postun
%systemd_postun_with_restart osbuild-composer.service osbuild-composer.socket osbuild-remote-worker.socket

%files
%license LICENSE
%doc README.md
%{_libexecdir}/osbuild-composer/osbuild-composer
%{_libexecdir}/osbuild-composer/dnf-json
%{_datadir}/osbuild-composer/
%{_unitdir}/osbuild-composer.service
%{_unitdir}/osbuild-composer.socket
%{_unitdir}/osbuild-remote-worker.socket
%{_sysusersdir}/osbuild-composer.conf

%package rcm
Summary:	RCM-specific version of osbuild-composer
Requires: 	osbuild-composer

%description rcm
RCM-specific version of osbuild-composer not intended for public usage.

%files rcm
%{_unitdir}/osbuild-rcm.socket

%post rcm
%systemd_post osbuild-rcm.socket

%preun rcm
%systemd_preun osbuild-rcm.socket

%postun rcm
%systemd_postun_with_restart osbuild-rcm.socket

%package tests
Summary:	Integration tests
Requires: 	osbuild-composer
Requires: 	composer-cli
Requires:	createrepo_c
Requires:   genisoimage
Requires:   qemu-kvm-core

%description tests
Integration tests to be run on a pristine-dedicated system to test the osbuild-composer package.

%files tests
%{_libexecdir}/tests/osbuild-composer/
%{_datadir}/tests/osbuild-composer/
%{_libexecdir}/osbuild-composer/image-info

%package worker
Summary:	The worker for osbuild-composer
Requires:   systemd
Requires:   osbuild

%description worker
The worker for osbuild-composer

%files worker
%{_libexecdir}/osbuild-composer/osbuild-worker
%{_unitdir}/osbuild-worker@.service
%{_unitdir}/osbuild-remote-worker@.service

%post worker
%systemd_post osbuild-worker@.service osbuild-remote-worker@.service

%preun worker
%systemd_preun osbuild-worker@.service osbuild-remote-worker@.service

%postun worker
%systemd_postun_with_restart osbuild-worker@.service osbuild-remote-worker@.service

%changelog
* Sun Dec 1 11:00:00 CEST 2019 Ondrej Budai <obudai@redhat.com> - 4-1
- New upstream release.
* Fri Nov 15 11:00:00 CEST 2019 Ondrej Budai <obudai@redhat.com> - 3-1
- New upstream release.
* Wed Nov 13 15:14:00 CEST 2019 Ondrej Budai <obudai@redhat.com> - 2-2
- Fix specfile according to packaging guidelines.
* Mon Nov 11 13:23:00 CEST 2019 Tom Gundersen <teg@jklm.no> - 2-1
- First release.

