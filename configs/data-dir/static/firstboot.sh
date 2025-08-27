#!/bin/sh
set -eux

# 1) Ensure basic tools
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
  git ca-certificates curl

# 2) Install ansible
apt-get install -y --no-install-recommends ansible-core

# 3) SSH keys
install -d -m 0700 /root/.ssh
cat >/root/.ssh/authorized_keys <<'EOF'
{{ .Params.ssh_authorized_key }}
EOF
chmod 0600 /root/.ssh/authorized_keys
chown -R root:root /root/.ssh

# 4) Kick off ansible-pull
ansible-pull -U {{ .Params.ansible_repo_url }} -C {{ .Params.ansible_branch }} {{ .Params.ansible_playbook | default("baremetal.yml") }} -e "enable_rollout=false" || true

# Optional: drop a marker so the service stops on success
touch /var/lib/firstboot.done
