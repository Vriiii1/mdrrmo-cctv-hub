#!/usr/bin/env bash
# scripts/setup.sh — one-shot bootstrap for a fresh CCX23 (Ubuntu 22.04 LTS).
# Idempotent: safe to re-run.
set -euo pipefail

if [[ "${UID}" -ne 0 ]]; then echo "must run as root" >&2; exit 1; fi

OPS_BASTION_IP="${OPS_BASTION_IP:?must set OPS_BASTION_IP}"
ACME_EMAIL="${ACME_EMAIL:?must set ACME_EMAIL}"

echo "[1/8] apt update + upgrade"
apt-get update -y
DEBIAN_FRONTEND=noninteractive apt-get -o Dpkg::Options::="--force-confnew" upgrade -y

echo "[2/8] base packages"
apt-get install -y curl ca-certificates gnupg ufw fail2ban unattended-upgrades htop tmux vim

echo "[3/8] swap (4 GB)"
if [[ ! -f /swapfile ]]; then
    fallocate -l 4G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

echo "[4/8] Docker"
if ! command -v docker >/dev/null; then
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list
    apt-get update -y
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
systemctl enable --now docker

echo "[5/8] ufw"
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow from "${OPS_BASTION_IP}" to any port 22 proto tcp
ufw allow 80/tcp     # ACME (LE HTTP-01)
ufw allow 443/tcp    # HTTPS (HLS, WebRTC, WHIP) — Caddy
ufw allow 8443/tcp   # RTSPS publish — Compose maps to MediaMTX:8322; auth via externalAuthenticationURL
ufw --force enable

echo "[6/8] SSH hardening"
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#*ChallengeResponseAuthentication.*/ChallengeResponseAuthentication no/' /etc/ssh/sshd_config
systemctl reload ssh

echo "[7/9] unattended-upgrades + fail2ban"
dpkg-reconfigure -fnoninteractive unattended-upgrades
systemctl enable --now fail2ban

echo "[8/9] nftables rate-limits (spec §2 NFRs)"
# 50 viewer-handshakes/sec/IP on 443; 10 publish-handshakes/sec/IP on 8443.
# Enforce at the kernel layer so the limit holds for both Path A and Path B
# (neither path bakes a Caddy rate-limit plugin into the Caddy image).
apt-get install -y nftables
cat > /etc/nftables.conf <<'NFT'
#!/usr/sbin/nft -f
flush ruleset
table inet mdrrmo_hub {
    set syn_443_v4 { type ipv4_addr; flags dynamic, timeout; size 65536; }
    set syn_8443_v4 { type ipv4_addr; flags dynamic, timeout; size 65536; }
    chain input {
        type filter hook input priority 0; policy accept;
        # Viewer handshakes: 50/sec/IP on 443, burst of 100, then drop.
        tcp dport 443 ct state new \
            update @syn_443_v4 { ip saddr timeout 1s limit rate over 50/second burst 100 packets } drop
        # Publish handshakes: 10/sec/IP on 8443, burst of 20.
        tcp dport 8443 ct state new \
            update @syn_8443_v4 { ip saddr timeout 1s limit rate over 10/second burst 20 packets } drop
    }
}
NFT
systemctl enable --now nftables
nft list ruleset | head -20

echo "[9/9] DONE"
docker --version
docker compose version
ufw status verbose
