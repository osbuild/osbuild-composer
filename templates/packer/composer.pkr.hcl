source "amazon-ebs" "image_builder" {

  # AWS settings.
  access_key = var.aws_access_key
  secret_key = var.aws_secret_key
  region = var.region

  # Use a static RHEL Cloud Access Image.
  source_ami          = "ami-0b0af3577fe5e3532"

  # Remove previous image before making the new one.
  force_deregister = true
  force_delete_snapshot = true

  # Ensure that the EBS snapshot used for the AMI meets our requirements.
  launch_block_device_mappings {
    delete_on_termination = "true"
    device_name           = "/dev/sda1"
    volume_size           = 25
    volume_type           = "gp2"
  }

  # Apply tags to the instance that is building our image.
  run_tags = {
    AppCode = "IMGB-001"
    Name = "packer-builder-for-${var.image_name}"
  }

  # Apply tags to the resulting AMI/EBS snapshot.
  tags = {
    AppCode = "IMGB-001"
    Name = "${var.image_name}"
    composer_commit = "${var.composer_commit}"
    osbuild_commit = "${var.osbuild_commit}"
  }

  # Set a name for the resulting AMI.
  ami_name = "${var.image_name}"

  # Network configuration for the instance building our image.
  associate_public_ip_address = true
  ssh_interface = "public_ip"
  ssh_username = "ec2-user"
  instance_type = "c5.large"
}

build {
  sources = ["source.amazon-ebs.image_builder"]

  provisioner "ansible" {
    playbook_file = "${path.root}/ansible/playbook.yml"
    user = "ec2-user"
    extra_arguments = [
      "-e", "COMPOSER_COMMIT=${var.composer_commit}",
      "-e", "OSBUILD_COMMIT=${var.osbuild_commit}",
      "--skip-tags", "${var.ansible_skip_tags}",
    ]
  }
}
