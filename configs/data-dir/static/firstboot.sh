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
apt-get update
apt-get install -y --no-install-recommends git ca-certificates curl ansible-core

# Set up SSH for Ansible (key provided by preseed late_command)
mkdir -p /root/.ssh && chmod 700 /root/.ssh
cat >/root/.ssh/authorized_keys <<'EOF'
{{ .Params.ssh_authorized_key }}
EOF
chmod 600 /root/.ssh/authorized_keys
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
