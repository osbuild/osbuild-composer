# Packer's HCL syntax doesn't have the same timestamp as the old Packer
# version.
locals {
    timestamp = regex_replace(timestamp(), "[- TZ:]", "")

    ami_name_append_pr = "testing-only"
    ami_full_name = "${var.ami_name}-${var.append_timestamp ? local.timestamp : local.ami_name_append_pr}"
}
