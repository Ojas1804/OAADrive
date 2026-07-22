#!/usr/bin/env bash
#
# One-shot installer for the Home Backup Server (MinIO + Postgres).
# Usage: sudo ./setup.sh
#
# What it does, with no manual steps required afterward:
#   1. Checks for Docker (and offers to install it if missing)
#   2. Creates data directories for MinIO and Postgres
#   3. Generates the docker-compose.yml, init.sql, and .env files
#   4. Generates strong random credentials automatically
#   5. Starts the stack and waits until both services report healthy
#   6. Creates the MinIO buckets (photos, backups, documents)
#   7. Installs a systemd service so everything survives a reboot
#   8. Prints a summary with the console URL and generated credentials

set -euo pipefail

# ---------- Config (edit these two if you want a different layout) ----------
INSTALL_DIR="${INSTALL_DIR:-/opt/homeserver}"
DATA_DIR="${DATA_DIR:-/opt/homeserver/data}"
# -----------------------------------------------------------------------------

REAL_USER="${SUDO_USER:-$USER}"

log()  { echo -e "\033[1;32m[setup]\033[0m $1"; }
warn() { echo -e "\033[1;33m[warn ]\033[0m $1"; }
die()  { echo -e "\033[1;31m[error]\033[0m $1"; exit 1; }

# ---------- 0. Must run as root (for systemd + /opt) ----------
if [[ $EUID -ne 0 ]]; then
  die "Please run this with sudo: sudo ./setup.sh"
fi

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

# ---------- 2. Create directories ----------
log "Creating install and data directories under $INSTALL_DIR"
mkdir -p "$INSTALL_DIR" "$DATA_DIR/minio-data" "$DATA_DIR/postgres-data"
chown -R "$REAL_USER":"$REAL_USER" "$INSTALL_DIR"

cd "$INSTALL_DIR"

# ---------- 3. Generate credentials (only if not already set up) ----------
if [[ ! -f .env ]]; then
  log "Generating secure random credentials..."
  MINIO_ROOT_USER="admin"
  MINIO_ROOT_PASSWORD="$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)"
  POSTGRES_USER="homeserver"
  POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)"
  POSTGRES_DB="homeserver"

  cat > .env <<EOF
MINIO_ROOT_USER=${MINIO_ROOT_USER}
MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD}
MINIO_DATA_PATH=${DATA_DIR}/minio-data

POSTGRES_USER=${POSTGRES_USER}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_DB=${POSTGRES_DB}
POSTGRES_DATA_PATH=${DATA_DIR}/postgres-data
EOF
  chmod 600 .env
  chown "$REAL_USER":"$REAL_USER" .env
else
  log ".env already exists — reusing existing credentials (idempotent run)."
fi

# shellcheck disable=SC1091
source .env

# ---------- 4. Write docker-compose.yml ----------
log "Writing docker-compose.yml"
cat > docker-compose.yml <<'EOF'
version: "3.9"

services:
  minio:
    image: minio/minio:latest
    container_name: minio
    restart: always
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD}
    volumes:
      - ${MINIO_DATA_PATH}:/data
    ports:
      - "9000:9000"
      - "9001:9001"
    networks:
      - backend
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 10s
      timeout: 5s
      retries: 5

  postgres:
    image: postgres:16
    container_name: postgres
    restart: always
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - ${POSTGRES_DATA_PATH}:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql:ro
    ports:
      - "5432:5432"
    networks:
      - backend
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5

networks:
  backend:
    driver: bridge
EOF

# ---------- 5. Write init.sql ----------
log "Writing init.sql"
cat > init.sql <<'EOF'
CREATE TABLE users (
    id                BIGSERIAL PRIMARY KEY,
    name              TEXT NOT NULL,
    email             TEXT NOT NULL UNIQUE,
    password_hash     TEXT NOT NULL,
    role              TEXT NOT NULL DEFAULT 'member',
    storage_quota_bytes BIGINT NOT NULL DEFAULT 5368709120,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE files (
    id                BIGSERIAL PRIMARY KEY,
    owner_id          BIGINT NOT NULL REFERENCES users(id),
    original_filename TEXT NOT NULL,
    minio_bucket      TEXT NOT NULL,
    minio_object_key  TEXT NOT NULL,
    size_bytes        BIGINT NOT NULL,
    mime_type         TEXT,
    checksum_sha256   TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE TABLE backup_jobs (
    id                BIGSERIAL PRIMARY KEY,
    owner_id          BIGINT NOT NULL REFERENCES users(id),
    source_path       TEXT NOT NULL,
    schedule          TEXT,
    last_run_at       TIMESTAMPTZ,
    status            TEXT NOT NULL DEFAULT 'pending'
);

CREATE TABLE audit_log (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT REFERENCES users(id),
    action            TEXT NOT NULL,
    target            TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_files_owner ON files(owner_id);
CREATE INDEX idx_backup_jobs_owner ON backup_jobs(owner_id);
EOF

# ---------- 6. Start the stack ----------
log "Starting Docker Compose stack..."
docker compose up -d

# ---------- 7. Wait for health ----------
log "Waiting for MinIO and Postgres to become healthy..."
for i in $(seq 1 30); do
  MINIO_STATUS=$(docker inspect --format='{{.State.Health.Status}}' minio 2>/dev/null || echo "starting")
  PG_STATUS=$(docker inspect --format='{{.State.Health.Status}}' postgres 2>/dev/null || echo "starting")
  if [[ "$MINIO_STATUS" == "healthy" && "$PG_STATUS" == "healthy" ]]; then
    log "Both services are healthy."
    break
  fi
  sleep 2
  if [[ $i -eq 30 ]]; then
    die "Services did not become healthy in time. Run 'docker compose logs' to debug."
  fi
done

# ---------- 8. Create MinIO buckets ----------
log "Creating MinIO buckets (photos, backups, documents)..."
docker run --rm --network host \
  -e "MC_HOST_local=http://${MINIO_ROOT_USER}:${MINIO_ROOT_PASSWORD}@localhost:9000" \
  minio/mc mb --ignore-existing local/photos local/backups local/documents

# ---------- 9. Install systemd service for auto-start on boot ----------
log "Installing systemd service for automatic startup on boot..."
cat > /etc/systemd/system/homeserver.service <<EOF
[Unit]
Description=Home Backup Server (Docker Compose stack)
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=${INSTALL_DIR}
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable homeserver.service
systemctl start homeserver.service

# ---------- 10. Summary ----------
LAPTOP_IP=$(hostname -I | awk '{print $1}')
CREDS_FILE="${INSTALL_DIR}/CREDENTIALS.txt"
cat > "$CREDS_FILE" <<EOF
Home Backup Server — generated credentials
Keep this file safe and delete it once you've saved the values elsewhere.

MinIO Console:  http://${LAPTOP_IP}:9001
MinIO User:     ${MINIO_ROOT_USER}
MinIO Password: ${MINIO_ROOT_PASSWORD}

Postgres DB:       ${POSTGRES_DB}
Postgres User:     ${POSTGRES_USER}
Postgres Password: ${POSTGRES_PASSWORD}
EOF
chmod 600 "$CREDS_FILE"
chown "$REAL_USER":"$REAL_USER" "$CREDS_FILE"

echo
log "Setup complete! ✅"
echo "  MinIO console: http://${LAPTOP_IP}:9001"
echo "  Credentials saved to: ${CREDS_FILE}"
echo "  The stack will now auto-start on every boot via systemd (homeserver.service)."
echo
echo "  Next: point your Go API's MinIO/Postgres client config at these credentials."
