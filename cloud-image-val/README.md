# cloud-image-val
Multi-cloud image validation tool. Right now it supports AWS and Azure.

# Dependencies
Apart from the python dependencies that can be found in `requirements.txt`, the environment where you will run this tool must have the following packages installed:

- Terraform: https://learn.hashicorp.com/tutorials/terraform/install-cli
- AWS cli: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html
- Azure cli (AKA "az"): https://docs.microsoft.com/en-us/cli/azure/install-azure-cli-linux?pivots=dnf

# Pre-requisites
Below you will find the specific requirements to make `cloud-image-val` tool work depending on the cloud provider.
The automation of some steps below will be automated in later versions of the tool (WIP).
### AWS
- You must have a working AWS account.
- The code is prepared to work with the **default profile** named `aws`.
- The credentials must be stored in `~/.aws/credentials` for the `[aws]` profile.
  - See https://docs.aws.amazon.com/sdk-for-php/v3/developer-guide/guide_credentials_profiles.html for more details.
- **Inbound rules** must be set to **allow SSH** connection from (at least) your public IP address in the default Security Group or VPC.

### Azure
- You must have a working Azure account.
- Be the **admin** of a **Resource Group** where all test VMs (and all dependant resources) will be deployed.
- Login to your Azure account by using the **az cli** before using `cloud-image-val` tool:
  - https://docs.microsoft.com/en-us/cli/azure/authenticate-azure-cli
- You must set up a **service principal** as per the following guide:
  - https://learn.hashicorp.com/tutorials/terraform/azure-build?in=terraform/azure-get-started#create-a-service-principal
- And **export** the corresponding environment variables as per the guide:
  - https://learn.hashicorp.com/tutorials/terraform/azure-build?in=terraform/azure-get-started#set-your-environment-variables


# Usage
Run the main script `cloud-image-val.py` with the corresponding and desired parameters (if applicable):

```
usage: cloud-image-val.py [-h] -r RESOURCES_FILE -o OUTPUT_FILE [-t TEST_FILTER] [-p] [-d]

options:
  -h, --help            show this help message and exit
  -r RESOURCES_FILE, --resources-file RESOURCES_FILE
                        Path to the resources_aws.json file that contains the Cloud provider and the images to use.
                        See cloud/terraform/sample/resources_<cloud>.json to know about the expected file structure.
  -o OUTPUT_FILE, --output-file OUTPUT_FILE
                        Output file path of the resultant Junit XML test report and others
  -t TEST_FILTER, --test-filter TEST_FILTER
                        Use this option to filter tests execution by test name
  -p, --parallel        Use this option to enable parallel test execution mode. Default is DISABLED
  -d, --debug           Use this option to enable debugging mode. Default is DISABLED
```
Example: `python cloud-image-val.py -r cloud/terraform/sample/resources_aws.json -o /tmp/report.xml -p -d`
