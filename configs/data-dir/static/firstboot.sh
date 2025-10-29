#!/bin/sh
set -eux

# Log everything for postmortem
exec > /var/log/firstboot.log 2>&1

# Idempotency: bail if we've already run
[ -f /var/lib/firstboot.done ] && exit 0

# Source defaults if present (written by preseed late_command)
[ -f /etc/default/firstboot ] && . /etc/default/firstboot

# Sensible defaults if the defaults file is missing/incomplete
: "${ANSIBLE_REPO_URL:=git@github.com:inngest/ansible.git}"
: "${ANSIBLE_BRANCH:=master}"
: "${ANSIBLE_PLAYBOOK:=baremetal.yml}"
: "${EXTRA_VARS:=enable_rollout=false}"
# ---------------------------------------------------------------------------------------------
# Configure networking: create a bridge 'br0' using the first detected 'ens*' interface
PORT=$(ip -o -br link | awk '{print $1}' | sed 's/:$//' | grep -E '^en' | grep -Ev 'v' | head -n1)
MAC=$(ifconfig ens2f0np0 | awk '/ether/{print $2}')

if [ -z "$PORT" ]; then
  exit 0
fi

# Backup existing config
cp -a /etc/network/interfaces /etc/network/interfaces.bak.$(date +%s) || true

# Write new config for bridge interface
cat >/etc/network/int-test <<EOF
# Loopback
auto lo
iface lo inet loopback

# Bridge with detected ens* PFs
auto br0
iface br0 inet dhcp
    bridge-ports ${PORT}
    dns-nameservers 1.1.1.1
    hwaddress ${MAC}

# Slaves should not get their own IP config
EOF

# Apply
ifreload -a || service networking restart
# ---------------------------------------------------------------------------------------------

export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y --no-install-recommends python3-pip git ca-certificates curl ansible-core ifupdown2

# Python dep used by netbox.netbox modules
python3 -m pip install --upgrade pip setuptools wheel
python3 -m pip install pynetbox requests
ansible-galaxy collection install netbox.netbox

# Set up SSH for Ansible (key provided by preseed late_command)
cat >> /root/.ssh/config <<'EOF'
Host github.com
  HostName github.com
  User git
  IdentityFile /root/.ssh/id_ansible
  IdentitiesOnly yes
EOF
chmod 644 /root/.ssh/config
chmod 600 /root/.ssh/authorized_keys /root/.ssh/id_ansible
chown -R root:root /root/.ssh

# Run the playbook from your repo
ansible-pull \
  -U "$ANSIBLE_REPO_URL" \
  -C "$ANSIBLE_BRANCH" \
  "$ANSIBLE_PLAYBOOK" \
  -e "$EXTRA_VARS" || true

# Mark complete and disable the service so it doesn't run again
touch /var/lib/firstboot.done
systemctl disable firstboot.service || true
systemctl daemon-reload || true
