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
# A list of users to share the AMI with
variable "image_users" {
  type = list(string)
  default = []
}

# Skip ansible tags
variable "ansible_skip_tags" {
  type = string
  default = ""
}
