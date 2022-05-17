import os
import json
from pprint import pprint

from cloud.terraform.aws_config_builder import AWSConfigBuilder
from cloud.terraform.azure_config_builder import AzureConfigBuilder


class TerraformConfigurator:
    supported_providers = ('aws', 'azure')

    main_tf = {'terraform': {'required_version': '>= 0.14.9'}}
    providers_tf = None
    resources_tf = None

    def __init__(self, ssh_key_path, resources_path):
        self.resources_path = resources_path
        self.ssh_key_path = ssh_key_path

        self.resources_dict = self._initialize_resources_dict()
        self.cloud_name = self.get_cloud_provider_from_resources()

    def _initialize_resources_dict(self):
        with open(self.resources_path) as f:
            return json.load(f)

    def get_cloud_provider_from_resources(self):
        if 'provider' not in self.resources_dict:
            raise Exception(f'No cloud providers found in {self.resources_path}')

        cloud_provider = self.resources_dict['provider']
        if cloud_provider not in self.supported_providers:
            raise Exception(f'Unsupported cloud provider: {cloud_provider}')

        return cloud_provider

    def configure_from_resources_json(self):
        self.build_configuration()
        self.save_configuration_to_json()

    def build_configuration(self):
        config_builder = self.get_config_builder()

        self.main_tf['terraform']['required_providers'] = config_builder.cloud_provider_definition

        self.providers_tf = config_builder.build_providers()
        self.resources_tf = config_builder.build_resources()

    def get_config_builder(self):
        cloud_name = self.resources_dict['provider']

        if cloud_name == 'aws':
            return AWSConfigBuilder(self.resources_dict, self.ssh_key_path)
        elif cloud_name == 'azure':
            return AzureConfigBuilder(self.resources_dict, self.ssh_key_path)

    def save_configuration_to_json(self):
        self.__dump_to_json(self.main_tf, 'main.tf.json')
        self.__dump_to_json(self.providers_tf, 'providers.tf.json')
        self.__dump_to_json(self.resources_tf, 'resources.tf.json')

    def __dump_to_json(self, content, file):
        with open(file, 'w') as config_file:
            json.dump(content, config_file)

    def print_configuration(self):
        pprint(self.main_tf)
        pprint(self.providers_tf)
        pprint(self.resources_tf)

    def remove_configuration(self):
        for file in ['main.tf.json', 'resources.tf.json', 'providers.tf.json']:
            if os.path.exists(file):
                os.remove(file)

    def get_username_by_instance_name(self, name):
        for instance in self.resources_dict['instances']:
            if instance['name'].replace('.', '-') == name:
                return instance['username']

        raise Exception(f'ERROR: No instance with name "{name}" was found')
