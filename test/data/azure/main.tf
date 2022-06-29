# azurerm version is hardcoded to prevent potential issues with new versions
terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "=2.56.0"
    }
  }
}

# Configure the Microsoft Azure Provider
provider "azurerm" {
  features {}
}

# Set necessary variables
variable "RESOURCE_GROUP" {
    type = string
}

variable "STORAGE_ACCOUNT" {
    type = string
}

variable "CONTAINER_NAME" {
    type = string
}

variable "BLOB_NAME" {
    type = string
}

variable "TEST_ID" {
    type = string
}

variable "HYPER_V_GEN" {
    type = string
}

# Use existing resource group
data "azurerm_resource_group" "testResourceGroup" {
  name = var.RESOURCE_GROUP
}

# Use existing storage blob
resource "azurerm_storage_blob" "testBlob" {
  name = var.BLOB_NAME
  storage_account_name = var.STORAGE_ACCOUNT
  storage_container_name = var.CONTAINER_NAME
  type = "Page"
  # The following is a workaround related to https://github.com/terraform-providers/terraform-provider-azurerm/issues/8392
  lifecycle {
    ignore_changes = [content_md5, source, parallelism, size]
  }
}

# Create vm image
resource "azurerm_image" "testimage" {
  name                = join("-", ["image", var.TEST_ID])
  tags = {
    gitlab-ci-test = "true"
  }
  location            = data.azurerm_resource_group.testResourceGroup.location
  resource_group_name = data.azurerm_resource_group.testResourceGroup.name
  hyper_v_generation = var.HYPER_V_GEN

  os_disk {
    os_type  = "Linux"
    os_state = "Generalized"
    blob_uri = azurerm_storage_blob.testBlob.url
    size_gb  = 20
  }
}

# Create virtual network
resource "azurerm_virtual_network" "testterraformnetwork" {
    name                = join("-", ["vnet", var.TEST_ID])
    tags = {
      gitlab-ci-test = "true"
    }
    address_space       = ["10.0.0.0/16"]
    location            = data.azurerm_resource_group.testResourceGroup.location
    resource_group_name = data.azurerm_resource_group.testResourceGroup.name

}

# Create subnet
resource "azurerm_subnet" "testterraformsubnet" {
    name                 = join("-", ["snet", var.TEST_ID])
    resource_group_name  = data.azurerm_resource_group.testResourceGroup.name
    virtual_network_name = azurerm_virtual_network.testterraformnetwork.name
    address_prefixes       = ["10.0.1.0/24"]
}

# Create public IPs
resource "azurerm_public_ip" "testterraformpublicip" {
    name                         = join("-", ["ip", var.TEST_ID])
    tags = {
      gitlab-ci-test = "true"
    }
    location                     = data.azurerm_resource_group.testResourceGroup.location
    resource_group_name          = data.azurerm_resource_group.testResourceGroup.name
    allocation_method            = "Dynamic"

}

# Create Network Security Group and rule
resource "azurerm_network_security_group" "testterraformnsg" {
    name                = join("-", ["nsg", var.TEST_ID])
    tags = {
      gitlab-ci-test = "true"
    }
    location            = data.azurerm_resource_group.testResourceGroup.location
    resource_group_name = data.azurerm_resource_group.testResourceGroup.name

    security_rule {
        name                       = "SSH"
        priority                   = 1001
        direction                  = "Inbound"
        access                     = "Allow"
        protocol                   = "Tcp"
        source_port_range          = "*"
        destination_port_range     = "22"
        source_address_prefix      = "*"
        destination_address_prefix = "*"
    }

}

# Create network interface
resource "azurerm_network_interface" "testterraformnic" {
    name                      = join("-", ["iface", var.TEST_ID])
    tags = {
      gitlab-ci-test = "true"
    }
    location                  = data.azurerm_resource_group.testResourceGroup.location
    resource_group_name       = data.azurerm_resource_group.testResourceGroup.name

    ip_configuration {
        name                          = "testNicConfiguration"
        subnet_id                     = azurerm_subnet.testterraformsubnet.id
        private_ip_address_allocation = "Dynamic"
        public_ip_address_id          = azurerm_public_ip.testterraformpublicip.id
    }

}

# Connect the security group to the network interface
resource "azurerm_network_interface_security_group_association" "test" {
    network_interface_id      = azurerm_network_interface.testterraformnic.id
    network_security_group_id = azurerm_network_security_group.testterraformnsg.id
}

# Create (and display) an SSH key
resource "tls_private_key" "test_ssh" {
  algorithm = "RSA"
  rsa_bits = 4096
}

output "tls_private_key" {
    value = tls_private_key.test_ssh.private_key_pem
    sensitive = true
}

# Create virtual machine
resource "azurerm_linux_virtual_machine" "testterraformvm" {
    name                  = join("-", ["vm", var.TEST_ID])
    tags = {
      gitlab-ci-test = "true"
    }
    location              = data.azurerm_resource_group.testResourceGroup.location
    resource_group_name   = data.azurerm_resource_group.testResourceGroup.name
    network_interface_ids = [azurerm_network_interface.testterraformnic.id]
    size                  = "Standard_B1s"
    custom_data           = filebase64("${path.module}/user-data")

    os_disk {
        name              = join("-", ["disk", var.TEST_ID])
        caching           = "ReadWrite"
        storage_account_type = "Standard_LRS"
    }

    source_image_id = azurerm_image.testimage.id
    computer_name  = "testvm"
    admin_username = "redhat"
    disable_password_authentication = true

    admin_ssh_key {
        username       = "redhat"
        public_key     = tls_private_key.test_ssh.public_key_openssh
    }

}

output "public_IP" {
    value = azurerm_linux_virtual_machine.testterraformvm.public_ip_address
}
