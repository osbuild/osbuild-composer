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
  ssh_interface = "public_ip"

  skip_create_ami=var.skip_create_ami
}

build {
  source "amazon-ebs.image_builder" {
    name = "rhel-9-x86_64"

    # Use a static RHEL 9.0 Cloud Access Image.
    source_ami = "ami-0f7c7d22de9e097ea"
    ssh_username = "ec2-user"
    instance_type = "c6a.large"
    aws_polling {
      delay_seconds = 20
      max_attempts  = 180
    }

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}-rhel-9-x86_64"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}"
      composer_commit = "${var.composer_commit}"
      os = "rhel"
      os_version = "9"
      arch = "x86_64"
    }

    # Ensure that the EBS snapshot used for the AMI meets our requirements.
    launch_block_device_mappings {
      delete_on_termination = "true"
      device_name           = "/dev/sda1"
      volume_size           = 10
      volume_type           = "gp3"
    }
  }

  source "amazon-ebs.image_builder" {
    name = "rhel-9-aarch64"

    # Use a static RHEL 9.0 Cloud Access Image.
    source_ami = "ami-019ece25c0f135889"
    ssh_username = "ec2-user"
    instance_type = "c6g.large"
    aws_polling {
      delay_seconds = 20
      max_attempts  = 180
    }

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}-rhel-9-aarch64"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}"
      composer_commit = "${var.composer_commit}"
      os = "rhel"
      os_version = "9"
      arch = "aarch64"
    }

    # Ensure that the EBS snapshot used for the AMI meets our requirements.
    launch_block_device_mappings {
      delete_on_termination = "true"
      device_name           = "/dev/sda1"
      volume_size           = 10
      volume_type           = "gp3"
    }
  }

  source "amazon-ebs.image_builder"  {
    name = "fedora-40-x86_64"

    # Fedora-Cloud-Base-AmazonEC2.x86_64-40-1.14-hvm-us-east-1-gp3-0
    source_ami = "ami-004f552bba0e5f64f"
    ssh_username = "fedora"
    instance_type = "c6a.large"

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}-fedora-40-x86_64"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}-fedora-40-x86_64"
      composer_commit = "${var.composer_commit}"
      os = "fedora"
      os_version = "40"
      arch = "x86_64"
    }

    # Ensure that the EBS snapshot used for the AMI meets our requirements.
    launch_block_device_mappings {
      delete_on_termination = "true"
      device_name           = "/dev/sda1"
      volume_size           = 6
      volume_type           = "gp3"
    }

    # go doesn't like modern Fedora crypto policies
    # see https://github.com/hashicorp/packer/issues/10074
    user_data = <<EOF
#!/bin/bash
update-crypto-policies --set LEGACY
EOF
  }

  source "amazon-ebs.image_builder"  {
    name = "fedora-40-aarch64"

    # Fedora-Cloud-Base-AmazonEC2.aarch64-40-1.14-hvm-us-east-1-gp3-0
    source_ami = "ami-0d3825b70fa928886"
    ssh_username = "fedora"
    instance_type = "c6g.large"

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}-fedora-40-aarch64"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}-fedora-40-aarch64"
      composer_commit = "${var.composer_commit}"
      os = "fedora"
      os_version = "40"
      arch = "aarch64"
    }

    # Ensure that the EBS snapshot used for the AMI meets our requirements.
    launch_block_device_mappings {
      delete_on_termination = "true"
      device_name           = "/dev/sda1"
      volume_size           = 6
      volume_type           = "gp3"
    }

    # go doesn't like modern Fedora crypto policies
    # see https://github.com/hashicorp/packer/issues/10074
    user_data = <<EOF
#!/bin/bash
update-crypto-policies --set LEGACY
EOF
  }

  # Ansible is a little broken on fedora>39, needs python-six
  provisioner "shell" {
    only = ["amazon-ebs.fedora-40-x86_64", "amazon-ebs.fedora-40-aarch64"]
    inline = [
      "sudo dnf install -y python3.9"
    ]
  }

  provisioner "ansible" {
    playbook_file = "${path.root}/ansible/playbook.yml"
    user = build.User
    extra_arguments = [
      "-e", "COMPOSER_COMMIT=${var.composer_commit}",
      "-e", "RH_ACTIVATION_KEY=${var.rh_activation_key}",
      "-e", "RH_ORG_ID=${var.rh_org_id}",
      "--tags", "${var.ansible_tags}",
    ]
    inventory_directory = "${path.root}/ansible/inventory/${source.name}"
  }
}
