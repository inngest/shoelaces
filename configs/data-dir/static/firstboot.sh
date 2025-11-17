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
: "${ANSIBLE_BRANCH:=main}"
: "${ANSIBLE_PLAYBOOK:=baremetal.yml}"
: "${EXTRA_VARS:='enable_rollout=false register_in_netbox=true'}"

export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y --no-install-recommends \
  python3-pip python3-venv git ca-certificates curl ansible-core ifupdown2 isc-dhcp-client bridge-utils

# ---------------------------------------------------------------------------------------------
# Configure networking: create a bridge 'br0' using the first detected 'en*' interface
PORT="$(ip -o -br link | grep -v lo | grep UP | awk '{print $1}')"

if [ -z "$PORT" ]; then
  echo "No active network interface found; skipping bridge setup"
else
  MAC="$(ip -o -br link | grep -v lo | grep UP | awk '{print $3}')"

  # Backup existing config
  cp -a /etc/network/interfaces "/etc/network/interfaces.bak.$(date +%s)" || true

  # Tear down existing bridge if present
  ifdown --force br0 || true
  pkill -f 'dhclient.*-6.*br0' || true
  pkill -f 'dhclient.*br0' || true
  ip -6 addr flush dev br0 || true
  ip -4 addr flush dev br0 || true
  rm -f /run/dhclient*.br0.pid /var/lib/dhcp/dhclient6.br0.leases || true

  # drop L3 from the slave and the bridge (fresh start)
  ip addr flush dev $PORT
  # ensure proper ensla ving order
  ip link set $PORT down || true

  # Write new config for bridge interface
  cat >/etc/network/interfaces.d/br0 <<EOF

# Bridge with detected PF
auto br0
iface br0 inet dhcp
    bridge-ports ${PORT}
    dns-nameservers 1.1.1.1 8.8.8.8
    hwaddress ether ${MAC}
EOF
  # Apply
  ifreload -a || service networking restart || true
  ifdown br0 || true
  ifup -v br0 || true
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

# Fixup the ansible key (Strip CRs and ensure a trailing newline)
tr -d '\r' < /root/.ssh/id_ansible > /root/.ssh/id_ansible.fix
printf '\n' >> /root/.ssh/id_ansible.fix
mv /root/.ssh/id_ansible.fix /root/.ssh/id_ansible

# Lock down SSH permissions
[ -f /root/.ssh/config ] && chmod 644 /root/.ssh/config
[ -f /root/.ssh/authorized_keys ] && chmod 600 /root/.ssh/authorized_keys || true
[ -f /root/.ssh/id_ansible ] && chmod 600 /root/.ssh/id_ansible || true
chmod 700 /root/.ssh
chown -R root:root /root/.ssh

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

# Install temporary vault password for ansible-vault
touch /root/.vault_pass
chmod 600 /root/.vault_pass
cat > /root/.vault_pass <<'PW'
VvNs0t1wyd7nmBR8gS31eY7K
PW
printf 'vault pass bytes: %s\n' "$(wc -c </root/.vault_pass)" || true

# install ansible-galaxy collections from the repo requirements file
mkdir -p /root/.ansible/pull
DEST=/root/.ansible/pull/ansible
ansible-pull -U git@github.com:inngest/ansible.git -C "$ANSIBLE_BRANCH" -d "$DEST" --accept-host-key -i localhost, -l localhost --vault-password-file /root/.vault_pass --full "$ANSIBLE_PLAYBOOK" --check || true
# After this ^^, the repo exists at $DEST even if --check fails
ansible-galaxy collection install -r "$DEST/collections/requirements.yml"

# Run the playbook
ansible-pull \
  -U "$ANSIBLE_REPO_URL" \
  -C "$ANSIBLE_BRANCH" \
  -d "$DEST" \
  --accept-host-key \
  -i localhost, -l localhost \
  -e "$EXTRA_VARS" \
  --vault-password-file /root/.vault_pass \
  "$ANSIBLE_PLAYBOOK" || true

# Remove vault password
# rm -f /root/.vault_pass

# In case the ansible playbook removed these packages, ensure they are installed
apt-get install -y isc-dhcp-client bridge-utils || true

# Mark complete and disable the service so it doesn't run again
touch /var/lib/firstboot.done
systemctl disable firstboot.service || true
systemctl daemon-reload || true
