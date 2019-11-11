%bcond_without check

%global goipath         github.com/osbuild/osbuild-composer

Version:        1

%gometa

%global common_description %{expand:
An image building service based on osbuild
It is inspired by lorax-composer and exposes the same API.
As such, it is a drop-in replacement.
}

Name:           %{goname}
Release:        1%{?dist}
Summary:        An image building service based on osbuild.

# Upstream license specification: Apache-2.0
License:        ASL 2.0
URL:            %{gourl}
Source0:        %{gosource}

BuildRequires:  systemd
BuildRequires:  golang(github.com/coreos/go-systemd/activation)
BuildRequires:  golang(github.com/google/uuid)
BuildRequires:  golang(github.com/julienschmidt/httprouter)

Requires: systemd
Requires: osbuild

%description
%{common_description}

%prep
%forgeautosetup -p1

%build
%gobuildroot
for cmd in cmd/* ; do
  %gobuild -o _bin/$(basename $cmd) %{goipath}/$cmd
done


%install
install -m 0755 -vd                                         %{buildroot}%{_prefix}/lib/osbuild-composer
install -m 0755 -vp _bin/*                                  %{buildroot}%{_prefix}/lib/osbuild-composer/
install -m 0644 -vp dnf-json                                %{buildroot}%{_prefix}/lib/osbuild-composer/

install -m 0755 -vd                                         %{buildroot}%{_unitdir}
install -m 0644 -vp distribution/*.{service,socket}         %{buildroot}%{_unitdir}/

install -m 0755 -vd                                         %{buildroot}%{_sysusersdir}
install -m 0644 -vp distribution/osbuild-composer.conf      %{buildroot}%{_sysusersdir}/

install -m 0755 -vd                                         %{buildroot}%{_localstatedir}/cache/osbuild-composer/dnf-cache

%if %{with check}
%check

# turn off modules
export GO111MODULE=off

# fix GOPATH, so that tests can found deps
export GOPATH=$(pwd)/_build:%{gopath}

%gotest ./...

%endif

%files
%license LICENSE
%doc README.md
%{_prefix}/lib/osbuild-composer/*
%{_unitdir}/*.{service,socket}
%{_sysusersdir}/osbuild-composer.conf

%changelog
* Mon Nov 11 13:23:00 CEST 2019 Tom Gundersen <teg@jklm.no> - 1-1
- First release.

