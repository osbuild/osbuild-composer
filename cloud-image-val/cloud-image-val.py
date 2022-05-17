import os

from argparse import ArgumentParser, RawTextHelpFormatter
from main.cloud_image_validator import CloudImageValidator


parser = ArgumentParser(formatter_class=RawTextHelpFormatter)

parser.add_argument('-r', '--resources-file',
                    help='Path to the resources_aws.json file that contains the Cloud provider and the images to use.\n'
                         'See cloud/terraform/sample/resources_<cloud>.json to know about the expected file structure.',
                    required=True)
parser.add_argument('-o', '--output-file',
                    help='Output file path of the resultant Junit XML test report and others',
                    required=True)
parser.add_argument('-t', '--test-filter',
                    help='Use this option to filter tests execution by test name',
                    default=None,
                    required=False)
parser.add_argument('-p', '--parallel',
                    help='Use this option to enable parallel test execution mode. Default is DISABLED',
                    default=False,
                    action='store_true',
                    required=False)
parser.add_argument('-d', '--debug',
                    help='Use this option to enable debugging mode. Default is DISABLED',
                    default=False,
                    action='store_true',
                    required=False)

if __name__ == '__main__':
    args = parser.parse_args()

    # Add current dir abspath to PYTHONPATH to avoid issues when importing modules
    if 'PYTHONPATH' not in os.environ:
        os.environ['PYTHONPATH'] = ''
    os.environ['PYTHONPATH'] = ':'.join([f'{os.path.dirname(__file__)}', os.environ['PYTHONPATH']])

    cloud_image_validator = CloudImageValidator(resources_file=args.resources_file,
                                                output_file=args.output_file,
                                                test_filter=args.test_filter,
                                                parallel=args.parallel,
                                                debug=args.debug)
    cloud_image_validator.main()
