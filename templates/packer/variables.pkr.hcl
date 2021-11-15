##############################################################################
## VARIABLES THAT ARE SAFE TO CHANGE 🤔

# Git SHA for the composer commit to include in the image.
variable "composer_commit" {
  type    = string
  default = "b075cac9e3d4a283918dbefa66cce46d372e3835"
}

# Git SHA for the osbuild commit to include in the image.
variable "osbuild_commit" {
  type    = string
  default = "f7bf23fabaae6027b1e1147b27870d90d4b1911f"
}

##############################################################################
## VARIABLES THAT PROBABLY SHOULD NOT BE CHANGED 😬

# AWS account configuration.
variable "aws_access_key" { type = string }
variable "aws_secret_key" { type = string }
variable "region" {
  type    = string
  default = "us-east-1"
}

# Automatically set by environment variables in GitHub Actions.
variable "imagebuilder_packer_sha" { type = string }

# Default name for images (will have timestamp appended by default).
variable "ami_name" {
  type = string
  default = "imagebuilder-service-image"
}

# Set to True to append a timestamp to the AMI name.
variable "append_timestamp" {
  type = bool
  default = true
}
