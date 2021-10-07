#!/bin/bash
killall -s SIGINT iostat
sleep 10
killall iostat || true
echo "PERFLOG $(date --rfc-3339=seconds) stopping iostat"
