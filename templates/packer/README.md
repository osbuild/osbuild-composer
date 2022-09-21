# osbuild-composer Packer configuration

This directory contains a packer configuration for building osbuild-composer
worker AMIs based on RHEL.

## Running packer locally

Run the following command in the root directory of this repository:
```
PKR_VAR_aws_access_key="" \
PKR_VAR_aws_secret_key="" \
PKR_VAR_image_name=YOUR_UNIQUE_IMAGE_NAME \
PKR_VAR_composer_commit=OSBUILD_COMPOSER_COMMIT_SHA \
PKR_VAR_osbuild_commit=OSBUILD_COMMIT_SHA \
  packer build templates/packer
```

## Launching an instance from the built AMI

The AMI expects that cloud-init is used to create a `/tmp/cloud_init_vars`
file that contains configuration values for the particular instance.

The following block shows an example of such a file. The order of the
key-value pairs is not fixed but all of them are required.

```
# Domain name of the composer instance that the worker connects to
COMPOSER_HOST=api.stage.openshift.com

# Port number of the composer instance that the worker connects to
COMPOSER_PORT=443

# AWS ARN of a secret containing a OAuth offline token that is used to authenticate to composer
# The secret contains only one key "offline_token". Its value is the offline token to be used. 
OFFLINE_TOKEN_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:offline-token-abcdef

# AWS ARN of a secret containing OAuth client credentials
# The secret contains two keys: "client_id" and "client_secret".
CLIENT_CREDENTIALS_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:client-credentials-abcdef

# Authentication URL to retrieve an access_token from
TOKEN_URL="https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"

# AWS ARN of a secret containing a command to subscribe the instance using subscription-manager
# The secrets contains only one key "subscription_manager_command" that contains the subscription-manager command
SUBSCRIPTION_MANAGER_COMMAND_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:subscription-manager-command-abcdef

# AWS ARN of a secret containing GCP service account credentials
# The secret contains a JSON key file, see https://cloud.google.com/docs/authentication/getting-started
GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:gcp_service_account_image_builder-abcdef

# AWS ARN of a secret containing Azure account credentials
# The secret contains two keys: "client_secret" and "client_id".
AZURE_ACCOUNT_IMAGE_BUILDER_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:azure_account_image_builder-abcdef

# AWS ARN of a secret containing AWS account credentials
# The secret contains two keys: "access_key_id" and "secret_access_key".
AWS_ACCOUNT_IMAGE_BUILDER_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:aws_account_image_builder-abcdef

# The auto-generated EC2 instance ID is prefixed with this string to simplify searching in logs
SYSTEM_HOSTNAME_PREFIX=staging-worker-aoc

# Endpoint URL for AWS Secrets Manager
SECRETS_MANAGER_ENDPOINT_URL=https://secretsmanager.us-east-1.amazonaws.com/

# Endpoint URL for AWS Cloudwatch Logs
CLOUDWATCH_LOGS_ENDPOINT_URL=https://logs.us-east-1.amazonaws.com/

# AWS Cloudwatch log group that the instance logs into
CLOUDWATCH_LOG_GROUP=staging_workers_aoc
```

### IAM considerations
The instance must have a IAM policy attached that permits it:

- to access all configured secrets
- to create new log streams in the configured log group and to put log entried in them


### Cloud-init example

The simplest way is to inject the file is to just use cloud-init's
`write_files` directive:

```
#cloud-config

write_files:
  - path: /tmp/cloud_init_vars
    content: |
      COMPOSER_HOST=api.stage.openshift.com
      COMPOSER_PORT=443 
      OFFLINE_TOKEN_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:offline-token-abcdef
      SUBSCRIPTION_MANAGER_COMMAND_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:subscription-manager-command-abcdef
      GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:gcp_service_account_image_builder-abcdef
      AZURE_ACCOUNT_IMAGE_BUILDER_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:azure_account_image_builder-abcdef
      AWS_ACCOUNT_IMAGE_BUILDER_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:aws_account_image_builder-abcdef
      SYSTEM_HOSTNAME_PREFIX=staging-worker-aoc
      SECRETS_MANAGER_ENDPOINT_URL=https://secretsmanager.us-east-1.amazonaws.com/
      CLOUDWATCH_LOGS_ENDPOINT_URL=https://logs.us-east-1.amazonaws.com/
      CLOUDWATCH_LOG_GROUP=staging_workers_aoc
      TOKEN_URL="https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"
```
