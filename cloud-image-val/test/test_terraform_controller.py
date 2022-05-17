import pytest
import os

from cloud.terraform.terraform_controller import TerraformController


class TestTerraformController:
    test_ssh_key = '/fake/ssh/dir'
    test_resources_path = '/fake/resources/path'

    def tf_configurator(self, mocker):
        tf_configurator = mocker.MagicMock()
        return tf_configurator

    @pytest.fixture
    def tf_controller(self, mocker):
        self.tf_configurator = self.tf_configurator(mocker)
        return TerraformController(self.tf_configurator)

    def test_create_infra(self, mocker, tf_controller):
        # Arrange
        mock_os_system = mocker.patch('os.system', return_value='')
        tf_init = f'terraform init {tf_controller.debug_sufix}'
        tf_apply = f'terraform apply -auto-approve {tf_controller.debug_sufix}'

        # Acts
        result = tf_controller.create_infra()

        # Assert
        assert result is None
        mock_os_system.assert_has_calls([mocker.call(tf_init), mocker.call(tf_apply)])

    @pytest.mark.parametrize(
        'cloud, test_instance',
        [('aws', 'test_instance_aws'),
         ('azure', 'test_instance_azure')])
    def test_get_instances(self, mocker, tf_controller, cloud, test_instance):
        # Arrange
        tf_controller.cloud_name = cloud
        test_resource = 'test_resource'

        mock_popen = mocker.patch('os.popen', return_value=os._wrap_close)

        mock_read = mocker.MagicMock(return_value='test_json')
        os._wrap_close.read = mock_read

        mock_loads = mocker.patch('json.loads',
                                  return_value={'values': {'root_module': {'resources': test_resource}}})

        test_instance = {'test_instance': 'test'}
        mock_get_instances_cloud = mocker.MagicMock(return_value=test_instance)
        tf_controller.get_instances_aws = mock_get_instances_cloud
        tf_controller.get_instances_azure = mock_get_instances_cloud

        # Act
        result = tf_controller.get_instances()

        # Assert
        mock_popen.assert_called_once_with('terraform show --json')
        mock_read.assert_called_once()
        mock_loads.called_once_with('test_json')
        mock_get_instances_cloud.assert_called_once_with(test_resource)

        assert result == test_instance

    def test_get_instances_aws(self, mocker, tf_controller):
        # Arrange
        resources = [
            {
                'type': 'aws_instance',
                'address': 'a.aws_instance_test',
                'name': 'test_name',
                'values': {
                    'id': 'test_id',
                    'public_ip': 'test_ip',
                    'public_dns': 'test_dns',
                    'availability_zone': 'test_zone',
                    'ami': 'test_ami',
                },
            },
            {'type': 'not_an_instance'},
        ]

        instances_info_expected = {
            'a.aws_instance_test': {
                'name': 'test_name',
                'instance_id': 'test_id',
                'public_ip': 'test_ip',
                'public_dns': 'test_dns',
                'availability_zone': 'test_zone',
                'ami': 'test_ami',
                'username': 'test_user',
            }
        }

        mock_get_username_by_instance_name = mocker.MagicMock(return_value='test_user')
        self.tf_configurator.get_username_by_instance_name = mock_get_username_by_instance_name

        # Act
        result = tf_controller.get_instances_aws(resources)

        # Assert
        mock_get_username_by_instance_name.assert_called_once_with('aws_instance_test')
        assert result == instances_info_expected

    def test_get_instances_azure(self, mocker, tf_controller):
        # Arrange
        test_name = 'test_name'
        test_computer_name = 'test_hostname'
        test_location = 'eastus'
        test_public_dns = f'{test_computer_name}.{test_location}.cloudapp.azure.com'
        test_public_ip = '10.11.12.13'
        test_resource_address = 'a.test_azure_address'
        test_id = '/subscription/xxx/test_azure_resource_id'
        test_username = 'azure'
        test_image_type = 'test_image_resource_type'
        test_image = 'test_image_data'

        resources = [
            {
                'type': 'azurerm_linux_virtual_machine',
                'address': test_resource_address,
                'name': test_name,
                'values': {
                    'id': test_id,
                    'computer_name': test_computer_name,
                    'admin_username': test_username,
                    'public_ip_address': test_public_ip,
                    'location': test_location,
                    test_image_type: test_image,
                },
            },
            {
                'type': 'not_a_vm_resource_type'
            },
        ]

        instances_info_expected = {
            test_resource_address: {
                'name': test_name,
                'instance_id': test_id,
                'public_ip': test_public_ip,
                'public_dns': test_public_dns,
                'location': test_location,
                'image': test_image,
                'username': test_username,
            }
        }

        mock_get_azure_image_data_from_resource = mocker.patch.object(tf_controller,
                                                                      '_get_azure_image_data_from_resource',
                                                                      return_value=test_image)

        # Act
        result = tf_controller.get_instances_azure(resources)

        # Assert
        assert result == instances_info_expected
        mock_get_azure_image_data_from_resource.assert_called_once_with(resources[0])

    @pytest.mark.parametrize(
        'test_image_type, test_image',
        [('source_image_reference', {'publisher': 'Canonical'}),
         ('source_image_id', '/test/image/uri')]
    )
    def test_get_azure_image_data_from_resource(self, tf_controller, test_image_type, test_image):
        test_resources = {'values': {test_image_type: test_image}}

        result = tf_controller._get_azure_image_data_from_resource(test_resources)

        assert result == test_image

    def test_destroy_resource(self, mocker, tf_controller):
        # Arrange
        mock_os_system = mocker.patch('os.system', return_value='')
        test_resource_id = 'test_resource'
        tf_destroy_resource = f'terraform destroy -target={test_resource_id}'

        # Act
        result = tf_controller.destroy_resource(test_resource_id)

        # Assert
        assert result is None
        mock_os_system.assert_called_once_with(tf_destroy_resource)

    def test_destroy_infra(self, mocker, tf_controller):
        # Arrange
        mock_os_system = mocker.patch('os.system', return_value='')
        tf_destroy_infra = f'terraform destroy -auto-approve {tf_controller.debug_sufix}'

        # Act
        result = tf_controller.destroy_infra()

        # Assert
        assert result is None
        mock_os_system.assert_called_once_with(tf_destroy_infra)
