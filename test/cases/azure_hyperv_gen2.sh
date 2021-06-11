#!/bin/bash
set -euo pipefail


# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/image#argument-reference
export HYPER_V_GEN="V2"
/usr/libexec/tests/osbuild-composer/azure.sh
