#!/bin/sh
set -eux

[ -f /etc/default/firstboot ] && . /etc/default/firstboot

apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends git ca-certificates curl
apt-get install -y --no-install-recommends ansible-core

mkdir -p /root/.ssh && chmod 700 /root/.ssh
cat >/root/.ssh/authorized_keys <<'EOF'
{{ .Params.ssh_authorized_key }}
EOF
chmod 600 /root/.ssh/authorized_keys
chown -R root:root /root/.ssh

ansible-pull \
  -U "${ANSIBLE_REPO_URL}" \
  -C "${ANSIBLE_BRANCH}" \
  "${ANSIBLE_PLAYBOOK}" \
  -e "enable_rollout=false" || true
touch /var/lib/firstboot.done
