# You need to specify all the following variables (except for region)
# when running `packer build`

# AWS account configuration.
variable "aws_access_key" {
  type = string
  default = ""
}
variable "aws_secret_key" {
  type = string
  default = ""
}
variable "region" {
  type    = string
  default = "us-east-1"
}

# cost center will end up in the tags
variable "cost_center" {
  type = string
  default = ""
}

# Automatically set by environment variables
variable "composer_commit" { type = string }

# Controls whether AMIs should be created. If you just need to test whether the image can be built, leave it as true
variable "skip_create_ami" {
  type = bool
  default = true
}

# The name of the resulting AMI and the underlying EBS snapshot
variable "image_name" { type = string }
# A list of users to share the AMI with
variable "image_users" {
  type = list(string)
  default = []
}

# Skip ansible tags
variable "ansible_tags" {
  type = string
  default = ""
}

# Subscription variables

variable "rh_org_id" {
  type = string
  default = ""
}

variable "rh_activation_key" {
  type = string
  default = ""
}
