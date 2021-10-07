#!/bin/bash
sudo dnf install -y sysstat
iostat -y -x -o JSON 5 > iostats.json &
echo "PERFLOG $(date --rfc-3339=seconds) starting iostat"
