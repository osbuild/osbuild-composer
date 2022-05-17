def is_rhel_atomic_host(host):
    with host.sudo():
        return host.file('/etc/redhat-release').contains('Atomic')


def is_rhel_sap(host):
    return __test_keyword_in_repositories(host, 'sap-bundle')


def is_rhel_high_availability(host):
    return __test_keyword_in_repositories(host, 'highavailability')


def __test_keyword_in_repositories(host, keyword):
    with host.sudo():
        if host.exists('yum'):
            return keyword in host.run('yum repolist 2>&1').stdout
