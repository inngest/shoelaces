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

# Install required packages
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

  # Write new config for bridge interface
  cat >/etc/network/interfaces.d/br0 <<EOF

# Bridge with detected PF
auto br0
iface br0 inet dhcp
    bridge-ports ${PORT}
    dns-nameservers 1.1.1.1 8.8.8.8
    hwaddress ether ${MAC}

iface br0 inet6 dhcp
    # v6 DNS; gateway comes from RA
    dns-nameservers 2606:4700:4700::1111 2001:4860:4860::8888
EOF
  # Apply
  ifdown br0 || true
  ifup -v br0 || true
fi

# print network interface br0 status and addresses
echo "=== Network interface br0 status ==="
ip addr show dev br0 || true
echo "\n=== DHCP leases for br0 ==="
cat /var/lib/dhcp/dhclient*.br0.leases || true
echo "=== Network interface $PORT status ==="
ip addr show dev "$PORT" || true
echo "\n=== DHCP leases for $PORT ==="
cat /var/lib/dhcp/dhclient*."$PORT".leases || true

# ------ Permanently set hostname to hyphen-delimited IP of br0 -----------------------------
# Prefer IPv4; fall back to non-link-local IPv6
IP4="$(ip -4 -o addr show dev br0 scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | head -n1 || true)"
IP6="$(ip -6 -o addr show dev br0 scope global 2>/dev/null | awk '!/ fe80:/{print $4}' | cut -d/ -f1 | head -n1 || true)"

if [ -n "$IP4" ]; then
  NEW_HOSTNAME="$(echo "$IP4" | tr '.' '-')" || true
elif [ -n "$IP6" ]; then
  # Replace colons with hyphens; squeeze repeats; lowercase
  NEW_HOSTNAME="$(echo "$IP6" | tr ':' '-' | tr -s '-' | tr 'A-Z' 'a-z')" || true
else
  # Fallback: random suffix so the script never blocks
  NEW_HOSTNAME="unknown-$(tr -dc a-z0-9 </dev/urandom | head -c6)" || true
fi

# Hostname rules: lowercase; keep it to 63 chars (single-label DNS limit)
NEW_HOSTNAME="$(echo "$NEW_HOSTNAME" | tr 'A-Z' 'a-z' | cut -c1-63)" || true

CURRENT_HOSTNAME="$(hostnamectl --static 2>/dev/null || true)"
if [ "$CURRENT_HOSTNAME" != "$NEW_HOSTNAME" ] && [ -n "$NEW_HOSTNAME" ]; then
  echo "$NEW_HOSTNAME" > /etc/hostname
  hostnamectl set-hostname "$NEW_HOSTNAME"

  # Ensure 127.0.1.1 maps to the new hostname (Debian convention)
  if grep -qE '^127\.0\.1\.1\b' /etc/hosts; then
    sed -i "s/^127\.0\.1\.1.*/127.0.1.1\t$NEW_HOSTNAME/g" /etc/hosts
  else
    printf "127.0.1.1\t%s\n" "$NEW_HOSTNAME" >> /etc/hosts
  fi
fi
unset HOSTNAME
export HOSTNAME="$(hostname -s)"
# ------ end hostname section ----------------------------------------------------------------

VENV=/opt/firstboot/.venv
python3 -m venv "$VENV"
. "$VENV/bin/activate"
pip install --upgrade pip setuptools wheel
pip install "ansible-core==2.14.*" pynetbox requests boto3 botocore

# ensure the venv's ansible is used as controller
export PATH="/opt/firstboot/.venv/bin:$PATH"
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

# Use venv Ansible everywhere
export PATH="/opt/firstboot/.venv/bin:$PATH"
export ANSIBLE_PYTHON_INTERPRETER="/opt/firstboot/.venv/bin/python"

ansible-galaxy collection install -r "$DEST/collections/requirements.yml"

# Run the playbook
ansible-pull \
  -U "$ANSIBLE_REPO_URL" \
  -C "$ANSIBLE_BRANCH" \
  -d "$DEST" \
  --accept-host-key \
  -i localhost, -l localhost \
  -e "enable_rollout=false" \
  -e "register_in_netbox=true" \
  -e "target_hostname=$NEW_HOSTNAME" \
  --vault-password-file /root/.vault_pass \
  "$ANSIBLE_PLAYBOOK" -vv || true

# Remove vault password
# rm -f /root/.vault_pass

# In case the ansible playbook removed these packages, ensure they are installed
apt-get install -y isc-dhcp-client bridge-utils || true

# Mark complete and disable the service so it doesn't run again
touch /var/lib/firstboot.done
systemctl disable firstboot.service || true
systemctl daemon-reload || true
