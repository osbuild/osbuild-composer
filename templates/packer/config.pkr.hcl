packer {
  required_plugins {
    amazon = {
      # we need to skip version 1.3.10 due to a bug
      # https://github.com/hashicorp/packer-plugin-amazon/issues/586
      # skip version 1.8.1
      # https://github.com/hashicorp/packer-plugin-amazon/issues/676
      version = ">= 1.2.3, != 1.3.10, != 1.8.1"
      source = "github.com/hashicorp/amazon"
    }
    ansible = {
      version = "~> 1"
      source = "github.com/hashicorp/ansible"
    }
  }
}
