#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Gold Token — Staging server bootstrap
#
# Run once on a fresh Ubuntu 22.04 VPS (DigitalOcean, Hetzner, etc.):
#   curl -sSL https://raw.githubusercontent.com/ismetaba/gold-token/main/scripts/setup-staging-server.sh | sudo bash
#
# Minimum recommended specs: 4 GB RAM, 2 vCPU, 40 GB SSD (~$24/month on DO)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

DEPLOY_USER="deploy"
APP_DIR="/opt/gold-token"
SECRETS_DIR="/etc/gold-token"

echo "=== Gold Token staging server setup ==="

# ── System update ─────────────────────────────────────────────────────────────
apt-get update -y
apt-get upgrade -y
apt-get install -y \
  ca-certificates curl gnupg lsb-release rsync ufw fail2ban

# ── Docker ───────────────────────────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -y
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
  systemctl enable --now docker
  echo "Docker installed."
else
  echo "Docker already installed, skipping."
fi

# ── deploy user ───────────────────────────────────────────────────────────────
if ! id "$DEPLOY_USER" &>/dev/null; then
  useradd --create-home --shell /bin/bash "$DEPLOY_USER"
  usermod -aG docker "$DEPLOY_USER"
  echo "User '$DEPLOY_USER' created and added to docker group."
fi

mkdir -p /home/"$DEPLOY_USER"/.ssh
chmod 700 /home/"$DEPLOY_USER"/.ssh

echo ""
echo ">>> IMPORTANT: paste the GitHub Actions deploy public key below,"
echo ">>> then press Ctrl+D when done:"
cat >> /home/"$DEPLOY_USER"/.ssh/authorized_keys
chmod 600 /home/"$DEPLOY_USER"/.ssh/authorized_keys
chown -R "$DEPLOY_USER":"$DEPLOY_USER" /home/"$DEPLOY_USER"/.ssh

# ── App directory ─────────────────────────────────────────────────────────────
mkdir -p "$APP_DIR"
chown -R "$DEPLOY_USER":"$DEPLOY_USER" "$APP_DIR"

# ── Secrets directory ─────────────────────────────────────────────────────────
mkdir -p "$SECRETS_DIR"
chmod 700 "$SECRETS_DIR"
chown root:root "$SECRETS_DIR"

echo ""
echo ">>> Generate JWT keys now? (y/n)"
read -r gen_jwt
if [[ "$gen_jwt" == "y" ]]; then
  openssl genrsa -out "$SECRETS_DIR/jwt_private_key.pem" 4096
  openssl rsa -in "$SECRETS_DIR/jwt_private_key.pem" \
    -pubout -out "$SECRETS_DIR/jwt_public_key.pem"
  chmod 600 "$SECRETS_DIR/jwt_private_key.pem"
  chmod 644 "$SECRETS_DIR/jwt_public_key.pem"
  echo "JWT keys written to $SECRETS_DIR/"
fi

# ── .env.staging ─────────────────────────────────────────────────────────────
ENV_FILE="$APP_DIR/.env.staging"
if [ ! -f "$ENV_FILE" ]; then
  echo ""
  echo ">>> Copy .env.staging.example from the repo to $ENV_FILE and fill in values."
  echo ">>> Then run: chmod 600 $ENV_FILE && chown deploy:deploy $ENV_FILE"
fi

# ── Firewall ──────────────────────────────────────────────────────────────────
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow 3000/tcp   # frontend
ufw allow 8081/tcp   # mint-burn (optional — close in production, use reverse proxy)
ufw allow 8082/tcp   # auth
ufw allow 8083/tcp   # price-oracle
ufw allow 8084/tcp   # wallet
ufw allow 8085/tcp   # order
ufw allow 8086/tcp   # compliance
ufw allow 8087/tcp   # kyc
ufw allow 8088/tcp   # por
# Prometheus/Grafana/Alertmanager are bound to 127.0.0.1 only — internal access via SSH tunnel
ufw --force enable
echo "Firewall configured."

# ── fail2ban ──────────────────────────────────────────────────────────────────
systemctl enable --now fail2ban
echo "fail2ban enabled."

echo ""
echo "=== Setup complete ==="
echo ""
echo "Next steps:"
echo "  1. Copy .env.staging.example to $APP_DIR/.env.staging and fill in all values"
echo "  2. Set STAGING_HOST and STAGING_SSH_KEY secrets in GitHub Actions"
echo "     (Settings → Environments → staging → Add secret)"
echo "  3. Set SLACK_WEBHOOK_URL secret for deploy notifications"
echo "  4. Push to main — the deploy-staging workflow will deploy automatically"
echo ""
echo "  Access Grafana via SSH tunnel:"
echo "    ssh -L 3001:127.0.0.1:3001 deploy@YOUR_VPS_IP"
echo "    Then open http://localhost:3001"
