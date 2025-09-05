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

REPO=$1
BRANCH=$2
PLAYBOOK=$3
ansible-pull -U $REPO -C $BRANCH $PLAYBOOK -e "enable_rollout=false" || true
touch /var/lib/firstboot.done
