source "amazon-ebs" "image_builder" {
  # AWS settings.
  access_key = var.aws_access_key
  secret_key = var.aws_secret_key
  region = var.region

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
    name = "rhel-10-x86_64"

    # For some reason there's no RHEL-10.1, though this one was rebuilt around the same time as the
    # arm one: RHEL-10.0.0_HVM-20251030-x86_64-0-Access2-GP3
    source_ami = "ami-0d2cf1078cac15da9"
    ssh_username = "ec2-user"
    instance_type = "c6a.large"
    aws_polling {
      delay_seconds = 20
      max_attempts  = 180
    }

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}-rhel-10-x86_64"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}"
      composer_commit = "${var.composer_commit}"
      os = "rhel"
      os_version = "10"
      arch = "x86_64"
      service-phase = "prod"
      app = "image-builder"
      managed_by_integration = "app-sre/infra"
      cost-center = "${var.cost_center}"
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
    name = "rhel-10-aarch64"

    # RHEL-10.1.0_HVM_GA-20251031-arm64-0-Access2-GP3
    source_ami = "ami-03d9eec0fe95df48d"
    ssh_username = "ec2-user"
    instance_type = "c6g.large"
    aws_polling {
      delay_seconds = 20
      max_attempts  = 180
    }

    # Set a name for the resulting AMI.
    ami_name = "${var.image_name}-rhel-10-aarch64"

    # Apply tags to the resulting AMI/EBS snapshot.
    tags = {
      AppCode = "IMGB-001"
      Name = "${var.image_name}"
      composer_commit = "${var.composer_commit}"
      os = "rhel"
      os_version = "10"
      arch = "aarch64"
      service-phase = "prod"
      app = "image-builder"
      managed_by_integration = "app-sre/infra"
      cost-center = "${var.cost_center}"
    }

    # Ensure that the EBS snapshot used for the AMI meets our requirements.
    launch_block_device_mappings {
      delete_on_termination = "true"
      device_name           = "/dev/sda1"
      volume_size           = 10
      volume_type           = "gp3"
    }
  }

  provisioner "ansible" {
    playbook_file = "${path.root}/ansible/playbook.yml"
    user = build.User
    extra_arguments = [
      "-e", "COMPOSER_COMMIT=${var.composer_commit}",
      "-e", "RH_ACTIVATION_KEY=${var.rh_activation_key}",
      "-e", "RH_ORG_ID=${var.rh_org_id}",
      "--tags", "${var.ansible_tags}",
      # Use legacy SCP protocol, instead of SFTP, to prevent issues like:
      # "Failed to connect to the host via scp: bash: line 1: /usr/lib/sftp-server: No such file or directory"
      "--scp-extra-args='-O'",
    ]
    inventory_directory = "${path.root}/ansible/inventory/${source.name}"
  }
}
