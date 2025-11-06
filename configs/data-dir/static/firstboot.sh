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

export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y --no-install-recommends \
  python3-pip python3-venv git ca-certificates curl ansible-core ifupdown2

# ---------------------------------------------------------------------------------------------
# Configure networking: create a bridge 'br0' using the first detected 'en*' interface
PORT="$(ip -o -br link | grep UP | grep -v lo | awk '{print $1}')"

if [ -z "$PORT" ]; then
  echo "No active network interface found; skipping bridge setup"
else
  MAC="$(ip -o -br link | grep UP | grep -v lo | awk '{print $3}')"
  
  # Backup existing config
  cp -a /etc/network/interfaces "/etc/network/interfaces.bak.$(date +%s)" || true

  # Write new config for bridge interface
  cat >/etc/network/interfaces.d/br0 <<EOF
# Loopback
auto lo
iface lo inet loopback

# Bridge with detected PF
auto br0
iface br0 inet dhcp
    bridge-ports ${PORT}
    dns-nameservers 1.1.1.1
    hwaddress ether ${MAC}
EOF
  # Apply
  ifreload -a || service networking restart
fi
# ---------------------------------------------------------------------------------------------

VENV=/opt/firstboot/.venv
python3 -m venv "$VENV"
. "$VENV/bin/activate"
pip install --upgrade pip setuptools wheel
pip install pynetbox requests
# Make ansible use the venv python when running modules
export ANSIBLE_PYTHON_INTERPRETER="$VENV/bin/python"

# NetBox collection (installed for ansible, OK to do system-wide)
ansible-galaxy collection install netbox.netbox

# Set up SSH for Ansible (key provided by preseed late_command)
mkdir -p /root/.ssh
chmod 700 /root/.ssh
if [ ! -f /root/.ssh/config ]; then
  cat >> /root/.ssh/config <<'EOF'
Host github.com
  HostName github.com
  User git
  IdentityFile /root/.ssh/id_ansible
  IdentitiesOnly yes
  StrictHostKeyChecking accept-new
EOF
fi

[ -f /root/.ssh/config ] && chmod 644 /root/.ssh/config
[ -f /root/.ssh/authorized_keys ] && chmod 600 /root/.ssh/authorized_keys || true
[ -f /root/.ssh/id_ansible ] && chmod 600 /root/.ssh/id_ansible || true
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
