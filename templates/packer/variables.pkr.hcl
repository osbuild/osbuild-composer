# You need to specify all the following variables (except for region)
# when running `packer build`

# AWS account configuration.
variable "aws_access_key" { type = string }
variable "aws_secret_key" { type = string }
variable "region" {
  type    = string
  default = "us-east-1"
}

# Automatically set by environment variables in GitHub Actions.
variable "composer_commit" { type = string }
variable "osbuild_commit" { type = string }

# The name of the resulting AMI and the underlying EBS snapshot
variable "image_name" { type = string }
