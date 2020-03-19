%global goipath         github.com/osbuild/osbuild-composer

Version:        8

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
BuildRequires:  golang(github.com/Azure/azure-storage-blob-go/azblob)
BuildRequires:  golang(github.com/BurntSushi/toml)
BuildRequires:  golang(github.com/coreos/go-semver/semver)
BuildRequires:  golang(github.com/coreos/go-systemd/activation)
BuildRequires:  golang(github.com/gobwas/glob)
BuildRequires:  golang(github.com/google/go-cmp/cmp)
BuildRequires:  golang(github.com/google/uuid)
BuildRequires:  golang(github.com/julienschmidt/httprouter)
BuildRequires:  golang(github.com/stretchr/testify/assert)
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

# The tests need to be compiled by `go test -c`.
# Go rpm macros have gotest, but it can be used only for running
# unit tests during rpmbuild.
#
# `go test -c` has very similar arguments to `go build`. The following
# macro uses that fact. It takes gobuild macro and substitutes `go build`
# with `go test -c` inside the macro. This way we get working `go test -c`
# command which should be in sync with changes in gobuild macro.
#
# Because gobuild macro gets expanded to a string containing apostrophes
# and shell expansions (which we don't want to expand just yet), heredoc
# is used to pipe the macro into sed. The apostrophes around HEREDOC
# put it into non-expansion mode.
%define gobuildtest(o:) %(sed "s/go build/go test -c/" <<'HEREDOC'
%{gobuild %{?**}}
HEREDOC
)

# the integration tests have the integration build tag, so it needs to be set
export BUILDTAGS="integration ${BUILDTAGS}"
%gobuildtest -o _bin/osbuild-tests %{goipath}/cmd/osbuild-tests
%gobuildtest -o _bin/osbuild-dnf-json-tests %{goipath}/cmd/osbuild-dnf-json-tests
%gobuildtest -o _bin/osbuild-weldr-tests %{goipath}/internal/weldrcheck/
%gobuildtest -o _bin/osbuild-rcm-tests %{goipath}/cmd/osbuild-rcm-tests
%gobuildtest -o _bin/osbuild-image-tests %{goipath}/cmd/osbuild-image-tests

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
# the changelog is distribution-specific, therefore it doesn't make sense to have it upstream
