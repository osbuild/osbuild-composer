packer {
  required_plugins {
    amazon = {
      # we need to skip version 1.3.10 due to a bug
      # https://github.com/hashicorp/packer-plugin-amazon/issues/586
      version = ">= 1.2.3, != 1.3.10"
      source = "github.com/hashicorp/amazon"
    }
  }
}
