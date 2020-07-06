config_opts['root'] = 'fedora-{{ releasever }}-{{ target_arch }}'
# config_opts['module_enable'] = ['list', 'of', 'modules']
# config_opts['module_install'] = ['module1/profile', 'module2/profile']

# fedora 31+ isn't mirrored, we need to run from koji

config_opts['chroot_setup_cmd'] = 'install @buildsys-build'

config_opts['dist'] = 'fc{{ releasever }}'  # only useful for --resultdir variable subst
config_opts['extra_chroot_dirs'] = [ '/run/lock', ]
config_opts['package_manager'] = 'dnf'
config_opts['bootstrap_image'] = 'fedora:{{ releasever }}'

config_opts['dnf.conf'] = """
[main]
keepcache=1
debuglevel=2
reposdir=/dev/null
logfile=/var/log/yum.log
retries=20
obsoletes=1
gpgcheck=0
assumeyes=1
syslog_ident=mock
syslog_device=
install_weak_deps=0
metadata_expire=0
best=1
module_platform_id=platform:f{{ releasever }}
protected_packages=

# repos

[fedora-internal]
name=Fedora Internal
baseurl=http://download-rdu01.fedoraproject.org/pub/fedora/linux/releases/$releasever/Everything/$basearch/os/
enabled=1
countme=1
metadata_expire=7d
repo_gpgcheck=0
type=rpm
gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-fedora-$releasever-$basearch
skip_if_unavailable=False
priority=5

[updates-internal]
name=Fedora Updates Internal
baseurl=http://download-rdu01.fedoraproject.org/pub/fedora/linux/updates/$releasever/Everything/$basearch/
enabled=1
countme=1
repo_gpgcheck=0
type=rpm
gpgcheck=1
metadata_expire=6h
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-fedora-$releasever-$basearch
skip_if_unavailable=False
priority=5

"""
