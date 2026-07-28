#!/usr/bin/env bash
#
# One-shot installer for the OAADrive home server stack
# (SeaweedFS + Postgres + Go API + React web), built from this repo.
#
# Usage: sudo ./setup.sh
# Run this from inside your clone of the OAADrive repo (the directory
# containing docker-compose.yml, Dockerfile, web/, cmd/, etc.).
#
# What it does, with no manual steps required afterward:
#   1. Checks for Docker (and offers to install it if missing)
#   2. Creates data directories for SeaweedFS and Postgres on the configured disk
#   3. Generates a .env file and seaweedfs-s3.json with strong random credentials (idempotent)
#   4. Builds the api/web images and starts the full stack
#   5. Waits until seaweedfs/postgres/api report healthy
#   6. Creates the SeaweedFS buckets (photos, backups, documents)
#   7. Installs a systemd service so everything survives a reboot
#   8. Prints a summary with URLs and generated credentials

set -euo pipefail

# ---------- Config ----------
# Where SeaweedFS/Postgres persist data. Point this at your external disk.
DATA_DIR="${DATA_DIR:-/opt/homeserver/data}"
# -----------------------------------------------------------------------------

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_USER="${SUDO_USER:-$USER}"

log()  { echo -e "\033[1;32m[setup]\033[0m $1"; }
warn() { echo -e "\033[1;33m[warn ]\033[0m $1"; }
die()  { echo -e "\033[1;31m[error]\033[0m $1"; exit 1; }

# ---------- 0. Must run as root (for systemd + data dirs) ----------
if [[ $EUID -ne 0 ]]; then
  die "Please run this with sudo: sudo ./setup.sh"
fi

for required in docker-compose.yml Dockerfile web/Dockerfile init.sql seaweedfs-s3.example.json; do
  [[ -f "${REPO_DIR}/${required}" ]] || die "Expected ${required} in ${REPO_DIR}. Run this script from inside the cloned repo."
done

# ---------- 1. Check / install Docker ----------
if ! command -v docker &>/dev/null; then
  log "Docker not found. Installing via get.docker.com..."
  curl -fsSL https://get.docker.com | sh
  usermod -aG docker "$REAL_USER"
  log "Docker installed. Added $REAL_USER to the docker group."
else
  log "Docker already installed: $(docker --version)"
fi

if ! docker compose version &>/dev/null; then
  die "Docker Compose plugin not found. Please install it and re-run this script."
fi

# ---------- 2. Create data directories ----------
log "Creating data directories under $DATA_DIR"
mkdir -p "$DATA_DIR/seaweedfs-data" "$DATA_DIR/postgres-data"
chown -R "$REAL_USER":"$REAL_USER" "$DATA_DIR"

cd "$REPO_DIR"

# ---------- 3. Generate credentials (only if not already set up) ----------
if [[ ! -f .env ]]; then
  log "Generating secure random credentials..."
  SEAWEEDFS_ACCESS_KEY="$(openssl rand -hex 10)"
  SEAWEEDFS_SECRET_KEY="$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-32)"
  POSTGRES_USER="homeserver"
  POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)"
  POSTGRES_DB="homeserver"
  JWT_SECRET="$(openssl rand -base64 48 | tr -d '/+=' | cut -c1-48)"

  cat > .env <<EOF
SEAWEEDFS_ACCESS_KEY=${SEAWEEDFS_ACCESS_KEY}
SEAWEEDFS_SECRET_KEY=${SEAWEEDFS_SECRET_KEY}
SEAWEEDFS_DATA_PATH=${DATA_DIR}/seaweedfs-data

POSTGRES_USER=${POSTGRES_USER}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_DB=${POSTGRES_DB}
POSTGRES_DATA_PATH=${DATA_DIR}/postgres-data

JWT_SECRET=${JWT_SECRET}
API_PORT=8080
WEB_PORT=3000
EOF
  chmod 600 .env
  chown "$REAL_USER":"$REAL_USER" .env
else
  log ".env already exists — reusing existing credentials (idempotent run)."
fi

# shellcheck disable=SC1091
source .env

# ---------- 3b. Generate seaweedfs-s3.json (S3 auth identity) ----------
if [[ ! -f seaweedfs-s3.json ]]; then
  log "Writing seaweedfs-s3.json with generated access/secret key..."
  cat > seaweedfs-s3.json <<EOF
{
  "identities": [
    {
      "name": "admin",
      "credentials": [
        {
          "accessKey": "${SEAWEEDFS_ACCESS_KEY}",
          "secretKey": "${SEAWEEDFS_SECRET_KEY}"
        }
      ],
      "actions": ["Admin", "Read", "Write"]
    }
  ]
}
EOF
  chmod 600 seaweedfs-s3.json
  chown "$REAL_USER":"$REAL_USER" seaweedfs-s3.json
else
  log "seaweedfs-s3.json already exists — reusing existing config (idempotent run)."
fi

# ---------- 4. Build and start the stack ----------
log "Building images (api, web)..."
docker compose build

log "Starting Docker Compose stack..."
docker compose up -d

# ---------- 5. Wait for health ----------
log "Waiting for seaweedfs, postgres, and api to become healthy..."
for i in $(seq 1 30); do
  SEAWEEDFS_STATUS=$(docker inspect --format='{{.State.Health.Status}}' seaweedfs 2>/dev/null || echo "starting")
  PG_STATUS=$(docker inspect --format='{{.State.Health.Status}}' postgres 2>/dev/null || echo "starting")
  API_STATUS=$(docker inspect --format='{{.State.Health.Status}}' api 2>/dev/null || echo "starting")
  if [[ "$SEAWEEDFS_STATUS" == "healthy" && "$PG_STATUS" == "healthy" && "$API_STATUS" == "healthy" ]]; then
    log "All services are healthy."
    break
  fi
  sleep 2
  if [[ $i -eq 30 ]]; then
    die "Services did not become healthy in time. Run 'docker compose logs' to debug."
  fi
done

# ---------- 6. Create SeaweedFS buckets ----------
# mc is a generic S3-compatible client, so it works against SeaweedFS's
# S3 gateway just as well as it does against MinIO.
log "Creating SeaweedFS buckets (photos, backups, documents)..."
docker run --rm --network host \
  -e "MC_HOST_local=http://${SEAWEEDFS_ACCESS_KEY}:${SEAWEEDFS_SECRET_KEY}@localhost:8333" \
  minio/mc mb --ignore-existing local/photos local/backups local/documents

# ---------- 7. Install systemd service for auto-start on boot ----------
log "Installing systemd service for automatic startup on boot..."
cat > /etc/systemd/system/homeserver.service <<EOF
[Unit]
Description=OAADrive Home Server (Docker Compose stack)
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=${REPO_DIR}
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable homeserver.service
systemctl start homeserver.service

# ---------- 8. Summary ----------
LAPTOP_IP=$(hostname -I | awk '{print $1}')
CREDS_FILE="${REPO_DIR}/CREDENTIALS.txt"
cat > "$CREDS_FILE" <<EOF
OAADrive Home Server — generated credentials
Keep this file safe and delete it once you've saved the values elsewhere.

Web app:              http://${LAPTOP_IP}:${WEB_PORT}
API:                  http://${LAPTOP_IP}:${API_PORT}
SeaweedFS Filer UI:   http://${LAPTOP_IP}:8888
SeaweedFS Master UI:  http://${LAPTOP_IP}:9333
SeaweedFS Access Key: ${SEAWEEDFS_ACCESS_KEY}
SeaweedFS Secret Key: ${SEAWEEDFS_SECRET_KEY}

Postgres DB:       ${POSTGRES_DB}
Postgres User:     ${POSTGRES_USER}
Postgres Password: ${POSTGRES_PASSWORD}

JWT Secret:        ${JWT_SECRET}
EOF
chmod 600 "$CREDS_FILE"
chown "$REAL_USER":"$REAL_USER" "$CREDS_FILE"

echo
log "Setup complete! ✅"
echo "  Web app:            http://${LAPTOP_IP}:${WEB_PORT}"
echo "  API:                http://${LAPTOP_IP}:${API_PORT}/healthz"
echo "  SeaweedFS filer UI: http://${LAPTOP_IP}:8888"
echo "  Credentials saved to: ${CREDS_FILE}"
echo "  The stack will now auto-start on every boot via systemd (homeserver.service)."
