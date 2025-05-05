#!/usr/bin/python

import re
import os
from pathlib import Path
from collections import defaultdict

# Step 1: Read from /tmp/file
with open("/tmp/check-snapshots.out", "r") as log_file:
    log_text = log_file.read()

# Step 2: Parse the log
target_file = None
replacement_map = defaultdict(dict)

for line in log_text.strip().splitlines():
    if line.strip().endswith('.json:'):
        target_file = line.strip().rstrip(':')
    elif "NEWER:" in line:
        match = re.search(r'NEWER: (.*) has a newer version - (.*)', line)
        if match:
            old, new = match.groups()
            replacement_map[target_file][old] = new

if target_file:
    file_path = Path(target_file)
    if file_path.exists():
        for old, new in replacement_map.items():
            ####COMMAND COMES OUT WRONG
            command = f"sed -i 's/{old}/{new}/g' {file_path.as_posix()}"
            os.system(command)
        print(f"Updated {file_path}")
    else:
        print(f"File {file_path} not found.")
else:
    print("No target file identified in the input") 
