import json
import re
import pytest

from lib import test_lib


@pytest.fixture
def instance_data_aws(host):
    instance_document_url = 'http://169.254.169.254/latest/dynamic/instance-identity/document'
    return json.loads(host.check_output(f'curl -s {instance_document_url}'))


class TestsAWS:
    def test_ami_name(self, host, instance_data):
        """
        Check there is 'RHEL' in RHEL AMIs.
        In the case of Red Hat SAP AMIs, check that they do not contain "Access2" in the AMI name.
        In the case of Red Hat High Availability AMIs, check that they are not ARM and the name does not contain "arm64"
        In the case of Fedora AMIs, check that it follows the right Fedora Cloud-Base name format.
        """
        distro = host.system_info.distribution
        ami_name = instance_data['name']

        if distro == 'rhel':
            assert 'RHEL' in ami_name, 'Expected "RHEL" in AMI name for Red Hat image'

            if test_lib.is_rhel_sap(host):
                assert 'SAP' in ami_name, 'Expected "SAP" in Red Hat SAP AMI name'
                assert 'Access2' not in ami_name, \
                    'The Access2 images are not needed for this SAP image set (RHELDST-4739)'

            if test_lib.is_rhel_high_availability(host):
                assert 'HA' in ami_name, 'Expected "HA" in Red Hat High Availability AMI name'
                assert host.system_info.arch != 'aarch64' and 'arm64' not in ami_name, \
                    'ARM (aarch64/arm64) is not supported in Red Hat High Availability images'

        elif distro == 'fedora':
            fedora_ami_name_format = re.compile(r'Fedora-Cloud-Base-[\d]{2}-[\d]{8}.n.[\d].(?:aarch64|x86_64)')
            assert re.match(fedora_ami_name_format, ami_name), \
                'Unexpected AMI name for Fedora image'

    def test_rh_cloud_firstboot_service_is_disabled(self, host):
        """
        Check that rh-cloud-firstboot is disabled.
        """
        assert not host.service('rh-cloud-firstboot').is_enabled, \
            'rh-cloud-firstboot service must be disabled'

        with host.sudo():
            cloud_firstboot_file = host.file('/etc/sysconfig/rh-cloud-firstboot')
            # TODO: Confirm if test should fail when this file does not exist
            if cloud_firstboot_file.exists:
                assert cloud_firstboot_file.contains('RUN_FIRSTBOOT=NO'), \
                    'rh-cloud-firstboot must be configured with RUN_FIRSTBOOT=NO'

    def test_iommu_strict_mode(self, host):
        """
        Use "iommu.strict=0" in ARM AMIs to get better performance.
        BugZilla 1836058
        """
        option = 'iommu.strict=0'

        with host.sudo():
            product_version = 8.5
            if float(host.system_info.release) < product_version \
                    and not test_lib.is_rhel_atomic_host(host):
                pytest.skip(f'Not applicable to RHEL AMIs earlier than {product_version}')

                iommu_option_present = host.file('/proc/cmdline').contains(option)

                if host.system_info.arch == 'x86_64':
                    assert not iommu_option_present, f'{option} must not be present in x86_64 AMIs'
                else:
                    assert iommu_option_present, f'{option} must be present in ARM AMIs'

    def test_nouveau_is_blacklisted(self, host):
        """
        Check that nouveau is disabled.
        BugZilla 1645772
        """
        product_version = float(host.system_info.release)

        if product_version < 7.0:
            pytest.skip('Not required in RHEL 6.x')

        with host.sudo():
            assert host.file('/proc/cmdline').contains('rd.blacklist=nouveau'), \
                'nouveau must be blacklisted in cmdline'

        if product_version < 8.5:
            file_to_check = '/etc/modprobe.d/blacklist-nouveau.conf'
        else:
            file_to_check = '/usr/lib/modprobe.d/blacklist-nouveau.conf'

        assert host.file(file_to_check).contains('blacklist nouveau'), \
            f'nouveau is not blacklisted in "{file_to_check}"'

    def test_unwanted_packages_are_not_present(self, host):
        """
        Some pkgs are not required in EC2.
        """
        unwanted_pkgs = [
            'aic94xx-firmware', 'alsa-firmware', 'alsa-lib', 'alsa-tools-firmware',
            'ivtv-firmware', 'iwl1000-firmware', 'iwl100-firmware', 'iwl105-firmware',
            'iwl135-firmware', 'iwl2000-firmware', 'iwl2030-firmware', 'iwl3160-firmware',
            'iwl3945-firmware', 'iwl4965-firmware', 'iwl5000-firmware', 'iwl5150-firmware',
            'iwl6000-firmware', 'iwl6000g2a-firmware', 'iwl6000g2b-firmware', 'iwl6050-firmware',
            'iwl7260-firmware', 'libertas-sd8686-firmware', 'libertas-sd8787-firmware', 'libertas-usb8388-firmware',
            'firewalld', 'biosdevname', 'plymouth', 'iprutils',
        ]

        if float(host.system_info.release) > 8.3:
            unwanted_pkgs.append('rng-tools')  # BugZilla 1888695

        if test_lib.is_rhel_sap(host):
            unwanted_pkgs.remove('alsa-lib')  # In RHEL SAP images, alsa-lib is allowed

        if test_lib.is_rhel_high_availability(host):
            unwanted_pkgs.append('rh-amazon-rhui-client')

        found_pkgs = [pkg for pkg in unwanted_pkgs if host.package(pkg).is_installed]

        assert len(found_pkgs) == 0, f'Found unexpected packages installed: {", ".join(found_pkgs)}'

    def test_required_packages_are_installed(self, host):
        """
        Some pkgs are required in EC2.
        https://kernel.googlesource.com/pub/scm/boot/dracut/dracut/+/18e61d3d41c8287467e2bc7178f32d188affc920%5E!/

        dracut-nohostonly -> dracut-config-generic
        dracut-norescue   -> dracut
                          -> dracut-config-rescue
        """
        required_pkgs = [
            'kernel', 'yum-utils', 'redhat-release', 'redhat-release-eula',
            'cloud-init', 'tar', 'rsync', 'dhcp-client', 'NetworkManager',
            'NetworkManager-cloud-setup', 'cloud-utils-growpart', 'gdisk',
            'insights-client', 'dracut-config-generic', 'dracut-config-rescue', 'grub2-tools',
        ]

        required_pkgs_v7 = [
            'kernel', 'yum-utils', 'cloud-init', 'dracut-config-generic',
            'dracut-config-rescue', 'grub2', 'tar', 'rsync', 'chrony'
        ]

        product_version = float(host.system_info.release)
        if product_version > 8.4:
            required_pkgs.remove('NetworkManager-cloud-setup')

        if 8.4 > product_version >= 8.0:
            required_pkgs.append('rng-tools')

        if test_lib.is_rhel_sap(host):
            required_pkgs.extend(['rhel-system-roles-sap', 'ansible'])

            required_pkgs.extend(['bind-utils', 'compat-sap-c++-9', 'nfs-utils', 'tcsh'])  # BugZilla 1959813

            required_pkgs.append('uuidd')  # BugZilla 1959813

            required_pkgs.extend(['cairo', 'expect', 'graphviz', 'gtk2',
                                  'iptraf-ng', 'krb5-workstation', 'libaio'])  # BugZilla 1959923, 1961168

            required_pkgs.extend(['libatomic', 'libcanberra-gtk2', 'libicu',
                                  'libpng12', 'libtool-ltdl', 'lm_sensors', 'net-tools'])  # BugZilla 1959923, 1961168

            required_pkgs.extend(['numactl', 'PackageKit-gtk3-module', 'xorg-x11-xauth', 'libnsl'])

            required_pkgs.append('tuned-profiles-sap-hana')  # BugZilla 1959962

        if test_lib.is_rhel_high_availability(host):
            required_pkgs.extend(['fence-agents-all', 'pacemaker', 'pcs'])

        if product_version < 8.0:
            required_pkgs = required_pkgs_v7

        missing_pkgs = [pkg for pkg in required_pkgs if not host.package(pkg).is_installed]

        assert len(missing_pkgs) == 0, f'Missing packages: {", ".join(missing_pkgs)}'

    def test_rhui_pkg_is_installed(self, host):
        if host.system_info.distribution == 'fedora':
            pytest.skip('Fedora AMIs do not require rhui pkg')

        unwanted_rhui_pkgs = None

        if test_lib.is_rhel_high_availability(host):
            required_rhui_pkg = 'rh-amazon-rhui-client-ha'
        elif test_lib.is_rhel_sap(host):
            required_rhui_pkg = 'rh-amazon-rhui-client-sap-bundle'
        else:
            required_rhui_pkg = 'rh-amazon-rhui-client'
            unwanted_rhui_pkgs = [
                'rh-amazon-rhui-client-ha',
                'rh-amazon-rhui-client-sap',
            ]

        if unwanted_rhui_pkgs:
            for pkg in unwanted_rhui_pkgs:
                assert not host.package(pkg).is_installed, \
                    f'Unexpected rhui package installed: {pkg}'

        assert host.package(required_rhui_pkg).is_installed, \
            f'Package "{required_rhui_pkg}" should be present'

    def test_amazon_timesync_service_is_used(self, host):
        """
        BugZilla 1679763
        """
        timesync_service_ipv4 = '169.254.169.123'
        ntp_leap_lines = [
            'leapsectz right/UTC',
            'pool 2.rhel.pool.ntp.org iburst',
        ]

        with host.sudo():
            chrony_conf_content = host.file('/etc/chrony.conf').content_string

            for line in ntp_leap_lines:
                commented_line_exists = re.match(f'#[ ]?{line}|#[ ]+{line}', chrony_conf_content) is not None

                assert f'server {timesync_service_ipv4}' in chrony_conf_content, \
                    f'chrony should point to Amazon Time Sync service IPv4 {timesync_service_ipv4}'

                compatible_version = 8.5
                if float(host.system_info.release) < compatible_version:
                    assert line not in chrony_conf_content or commented_line_exists, \
                        f'NTP leap smear incompatibility found in chrony conf file, ' \
                        f'affecting RHEL lower than {compatible_version}'
                else:
                    assert line in chrony_conf_content and not commented_line_exists, \
                        f'{line} must be enabled in RHEL {compatible_version} and above'

            assert f'Selected source {timesync_service_ipv4}' in host.check_output('journalctl -u chronyd'), \
                'Amazon Time Sync service is not in use'

    def test_max_cstate_is_configured_in_cmdline(self, host):
        """
        Check that intel_idle.max_cstate=1 processor.max_cstate=1 exists in SAP AMI's /proc/cmdline.
        BugZilla 1961225
        """
        cstate_setting_lines = [
            'intel_idle.max_cstate=1',
            'processor.max_cstate=1',
        ]

        with host.sudo():
            for line in cstate_setting_lines:
                if test_lib.is_rhel_sap(host):
                    assert host.file('/proc/cmdline').contains(line), \
                        f'{line} must be specified in SAP AMIs'
                else:
                    assert not host.file('/proc/cmdline').contains(line), \
                        f'{line} must not be specified in AMIs that are not SAP'

    def test_hostkey_permissions(self, host):
        """
        Check that ssh files permission set are correct.
        BugZilla 2013644
        """
        files_to_check = ['ssh_host_ecdsa_key', 'ssh_host_ed25519_key', 'ssh_host_rsa_key']
        for file in files_to_check:
            if host.file(f'/etc/ssh/{file}').exists:
                assert host.file(f'/etc/ssh/{file}').mode >= 0o640, \
                    'ssh files permissions are not set correctly'

    def test_aws_instance_identity(self, host, instance_data, instance_data_aws):
        """
        Try to fetch instance identity from EC2 and compare with expectation
        """
        assert instance_data_aws['imageId'] == instance_data['ami'], \
            'Unexpected AMI ID for deployed instance'

        assert instance_data_aws['region'] in instance_data['availability_zone'], \
            'Unexpected region for deployed instance'

        arch = instance_data_aws['architecture']
        if arch == 'arm64':
            arch = 'aarch64'

        assert arch == host.system_info.arch, \
            'Unexpected architecture for deployed instance'

        if host.system_info.distribution == 'fedora':
            pytest.skip('No need to check billing codes in Fedora AMIs')

        ami_name = instance_data['name']

        billing_codes = []
        if test_lib.is_rhel_high_availability(host) and 'Access2' not in ami_name:
            # RHELDST-4222, on-demand (hourly) has the billing code for RHEL and for HA
            billing_codes = ['bp-79a54010', 'bp-6fa54006']
        elif 'Hourly2' in ami_name:
            billing_codes = ['bp-6fa54006']
        elif 'Access2' in ami_name:
            # Cloud Access billing code, means don't charge for the OS (so it can apply to anything cloud Access)
            billing_codes = ['bp-63a5400a']
        else:
            pytest.skip('Unable to decide billing codes as no "Hourly2" or "Access2" found in AMI name')

        for code in billing_codes:
            assert code in instance_data_aws['billingProducts'], \
                'Expected billing code not found in instance document data'


class TestsAWSNetworking:
    def test_correct_network_driver_is_used(self, host):
        """
        If ena network device found, eth0 should use ena as default driver.
        If vf network device found, eth0 should use ixgbevf as default driver.
        If others, eth0 should use vif as default driver.
        If it is not a xen instance, ena should be used.
        """
        with host.sudo():
            if host.system_info.distribution == 'fedora':
                host.run_test('dnf install lshw -y')

            nic_name = host.check_output('lshw -C network')
            nic_driver = host.check_output('lshw -C network | grep "driver="')

            self.__test_nic_is_using_correct_driver(nic_name, nic_driver)

            if 'Xen' not in host.check_output('lscpu'):
                assert 'ena' in nic_driver, 'ENA driver must de used in KVM, arch64 and metal instances'

    def __test_nic_is_using_correct_driver(self, nic_name, nic_driver):
        name_filter, driver_filter = self.__get_nic_filters_for_drivers(nic_name)

        assert driver_filter in nic_driver, \
            f'{name_filter} network adapter must use {driver_filter} driver'

    def __get_nic_filters_for_drivers(self, nic_name):
        if 'ENA' in nic_name:
            nic_name_filter = 'ENA'
            nic_driver_name_filter = 'ena'
        elif 'Virtual Function' in nic_name:
            nic_name_filter = 'Virtual Function'
            nic_driver_name_filter = 'ixgbevf'
        else:
            nic_name_filter = 'Other'
            nic_driver_name_filter = 'vif'

        return nic_name_filter, nic_driver_name_filter

    def test_network_ipv6_setup(self, host):
        """
        Check for IPv6 networking setup.
        """
        mac_addresses_url = 'http://169.254.169.254/latest/meta-data/network/interfaces/macs'
        registered_mac_address = host.check_output(f'curl -s {mac_addresses_url}').replace('/', '')
        registered_ipv6 = host.check_output(f'curl -s {mac_addresses_url}/{registered_mac_address}/ipv6s')

        if 'Not Found' in registered_ipv6:
            pytest.skip('No IPv6 enabled in this Subnet')

        assert registered_ipv6 in host.interface('eth0', 'inet6').addresses(), \
            f'Expected IPv6 {registered_ipv6} is not being used by eth0 network adapter'

    def test_redhat_cds_hostnames(self, host, instance_data_aws):
        """
        Check all Red Hat CDS for the AMI's instance region.
        """
        region = instance_data_aws['region']

        rhui_cds_hostnames = [
            f'rhui2-cds01.{region}.aws.ce.redhat.com',
            f'rhui2-cds02.{region}.aws.ce.redhat.com',
            f'rhui3-cds01.{region}.aws.ce.redhat.com',
            f'rhui3-cds02.{region}.aws.ce.redhat.com',
            f'rhui3-cds03.{region}.aws.ce.redhat.com',
        ]

        with host.sudo():
            for cds in rhui_cds_hostnames:
                # There is no rhui in us-gov regions at all.
                # All the content requests are redirected to the closest standard regions.
                cds_name = cds.replace('-gov', '')

                assert host.run_test(f'getent hosts {cds_name}'), \
                    f'Error getting {cds_name} host entry'
