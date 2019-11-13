%global goipath         github.com/osbuild/osbuild-composer

Version:        2

%gometa

%global common_description %{expand:
An image building service based on osbuild
It is inspired by lorax-composer and exposes the same API.
As such, it is a drop-in replacement.
}

Name:           %{goname}
Release:        2%{?dist}
Summary:        An image building service based on osbuild

# Upstream license specification: Apache-2.0
License:        ASL 2.0
URL:            %{gourl}
Source0:        %{gosource}

BuildRequires:  systemd-rpm-macros
BuildRequires:  systemd
BuildRequires:  golang(github.com/coreos/go-systemd/activation)
BuildRequires:  golang(github.com/google/uuid)
BuildRequires:  golang(github.com/julienschmidt/httprouter)

Requires: systemd
Requires: osbuild

%description
%{common_description}

%prep
%goprep

%build
for cmd in cmd/* ; do
  %gobuild -o _bin/$(basename $cmd) %{goipath}/$cmd
done

%install
install -m 0755 -vd                                         %{buildroot}%{_libexecdir}/osbuild-composer
install -m 0755 -vp _bin/*                                  %{buildroot}%{_libexecdir}/osbuild-composer/
install -m 0755 -vp dnf-json                                %{buildroot}%{_libexecdir}/osbuild-composer/

install -m 0755 -vd                                         %{buildroot}%{_unitdir}
install -m 0644 -vp distribution/*.{service,socket}         %{buildroot}%{_unitdir}/

install -m 0755 -vd                                         %{buildroot}%{_sysusersdir}
install -m 0644 -vp distribution/osbuild-composer.conf      %{buildroot}%{_sysusersdir}/

install -m 0755 -vd                                         %{buildroot}%{_localstatedir}/cache/osbuild-composer/dnf-cache

%check
%gocheck

%post
%systemd_post osbuild-composer.service osbuild-composer.socket osbuild-worker@.service

%preun
%systemd_preun osbuild-composer.service osbuild-composer.socket osbuild-worker@.service

%postun
%systemd_postun_with_restart osbuild-composer.service osbuild-composer.socket osbuild-worker@.service

%files
%license LICENSE
%doc README.md
%{_libexecdir}/osbuild-composer/
%{_unitdir}/*.{service,socket}
%{_sysusersdir}/osbuild-composer.conf

%changelog
* Wed Nov 13 15:14:00 CEST 2019 Ondrej Budai <obudai@redhat.com> - 2-2
- Fix specfile according to packaging guidelines.
* Mon Nov 11 13:23:00 CEST 2019 Tom Gundersen <teg@jklm.no> - 2-1
- First release.

