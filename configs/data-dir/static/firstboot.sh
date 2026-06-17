#!/bin/sh
set -eu

# Example firstboot hook. Operators can replace this disk-backed file with
# site-specific bootstrap logic and call it from a late_command partial.
exec > /var/log/firstboot.log 2>&1

echo "firstboot example started"
date -u

touch /var/lib/firstboot.done
echo "firstboot example completed"
