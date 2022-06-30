source "amazon-ebs" "image_builder" {
  # AWS settings.
  access_key = var.aws_access_key
  secret_key = var.aws_secret_key
  region = var.region

  # Remove previous image before making the new one.
  force_deregister = true
  force_delete_snapshot = true

  # Apply tags to the instance that is building our image.
  run_tags = {
    AppCode = "IMGB-001"
    Name = "packer-builder-for-${var.image_name}-${source.name}"
  }

  # Share the resulting AMI with accounts
  ami_users = "${var.image_users}"

  # Network configuration for the instance building our image.
  associate_public_ip_address = true
  ssh_interface = "public_ip"

  skip_create_ami=var.skip_create_ami
}

build {
  source "amazon-ebs.image_builder" {
    name = "rhel-8-x86_64"

    # Use a static RHEL 8.6 Cloud Access Image.
    source_ami = "ami-03debf3ebf61b20cd"
    ssh_username = "ec2-user"
    instance_type = "c6a.large"

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}"
      composer_commit = "${var.composer_commit}"
      os = "rhel"
      os_version = "8"
      arch = "x86_64"
    }

    # Ensure that the EBS snapshot used for the AMI meets our requirements.
    launch_block_device_mappings {
      delete_on_termination = "true"
      device_name           = "/dev/sda1"
      volume_size           = 10
      volume_type           = "gp2"
    }
  }

  source "amazon-ebs.image_builder"  {
    name = "fedora-35-x86_64"

    # Use a static Fedora 35 Cloud Base Image.
    source_ami = "ami-08b4ee602f76bff79"
    ssh_username = "fedora"
    instance_type = "c6a.large"

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}-fedora-35-x86_64"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}-fedora-35-x86_64"
      composer_commit = "${var.composer_commit}"
      os = "fedora"
      os_version = "35"
      arch = "x86_64"
    }

    # Ensure that the EBS snapshot used for the AMI meets our requirements.
    launch_block_device_mappings {
      delete_on_termination = "true"
      device_name           = "/dev/sda1"
      volume_size           = 5
      volume_type           = "gp2"
    }

    # go doesn't like modern Fedora crypto policies
    # see https://github.com/hashicorp/packer/issues/10074
    user_data = <<EOF
#!/bin/bash
update-crypto-policies --set LEGACY
EOF
  }

  source "amazon-ebs.image_builder"  {
    name = "fedora-35-aarch64"

    # Use a static Fedora 35 Cloud Base Image.
    source_ami = "ami-068c123e1c1ca0d49"
    ssh_username = "fedora"
    instance_type = "c6g.large"

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}-fedora-35-aarch64"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}-fedora-35-aarch64"
      composer_commit = "${var.composer_commit}"
      os = "fedora"
      os_version = "35"
      arch = "aarch64"
    }

    # Ensure that the EBS snapshot used for the AMI meets our requirements.
    launch_block_device_mappings {
      delete_on_termination = "true"
      device_name           = "/dev/sda1"
      volume_size           = 5
      volume_type           = "gp2"
    }

    # go doesn't like modern Fedora crypto policies
    # see https://github.com/hashicorp/packer/issues/10074
    user_data = <<EOF
#!/bin/bash
update-crypto-policies --set LEGACY
EOF
  }

  provisioner "ansible" {
    playbook_file = "${path.root}/ansible/playbook.yml"
    user = build.User
    extra_arguments = [
      "-e", "COMPOSER_COMMIT=${var.composer_commit}",
      "--skip-tags", "${var.ansible_skip_tags}",
    ]
    inventory_directory = "${path.root}/ansible/inventory/${source.name}"
  }
}
