from cloud.terraform.base_config_builder import BaseConfigBuilder


class AWSConfigBuilder(BaseConfigBuilder):
    cloud_name = 'aws'
    cloud_provider_definition = {'aws': {'source': 'hashicorp/aws', 'version': '~> 3.27'}}

    def __init__(self, resources_dict, ssh_key_path):
        super().__init__(resources_dict)

        self.ssh_key_path = ssh_key_path

    def build_providers(self):
        all_regions = self.__get_all_regions_from_resources_file()
        for region in all_regions:
            self.providers_tf['provider'][self.cloud_providers[self.cloud_name]]\
                .append(self.__new_aws_provider(region))

        return self.providers_tf

    def __get_all_regions_from_resources_file(self):
        instances_regions = [i['region'] for i in self.resources_dict['instances']]

        return list(dict.fromkeys(instances_regions))

    def __new_aws_provider(self, region, aws_profile='aws'):
        return {
            'region': region,
            'alias': region,
            'profile': aws_profile,
        }

    def build_resources(self):
        self.resources_tf['resource']['aws_key_pair'] = {}
        self.resources_tf['resource']['aws_instance'] = {}

        for instance in self.resources_dict['instances']:
            self.__new_aws_key_pair(instance['region'])
            self.__new_aws_instance(instance)

        return self.resources_tf

    def __new_aws_key_pair(self, region):
        key_name = f'{region}-key'

        new_key_pair = {
            'provider': f'aws.{region}',
            'key_name': key_name,
            'public_key': f'${{file("{self.ssh_key_path}")}}',
        }

        self.resources_tf['resource']['aws_key_pair'][key_name] = new_key_pair

    def __new_aws_instance(self, instance):
        if not instance['instance_type']:
            instance['instance_type'] = 't2.micro'

        name = instance['name'].replace('.', '-')

        aliases = [provider['alias'] for provider in self.providers_tf['provider'][self.cloud_name]]
        if instance['region'] not in aliases:
            raise Exception('Cannot add an instance if region provider is not set up')

        key_name = f'{instance["region"]}-key'

        new_instance = {
            'instance_type': instance['instance_type'],
            'ami': instance['ami'],
            'provider': f'aws.{instance["region"]}',
            'key_name': key_name,
            'tags': {'name': name},
            'depends_on': [f'aws_key_pair.{key_name}']
        }

        self.resources_tf['resource']['aws_instance'][name] = new_instance
