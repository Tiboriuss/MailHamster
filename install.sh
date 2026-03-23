#!/usr/bin/env bash
# MailHamster installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Tiboriuss/MailHamster/main/install.sh | sudo bash
set -euo pipefail

REPO="Tiboriuss/MailHamster"
INSTALL_BIN="/usr/local/bin/mailhamster"
CONF_DIR="/etc/mailhamster"
CONF_FILE="${CONF_DIR}/mailhamster.yaml"
SERVICE_FILE="/etc/systemd/system/mailhamster.service"
SERVICE_NAME="mailhamster"
SERVICE_USER="mailhamster"

# ── helpers ──────────────────────────────────────────────────────────────────
die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }
need() { command -v "$1" &>/dev/null || die "required tool not found: $1 (please install it and retry)"; }

# ── pre-flight checks ────────────────────────────────────────────────────────
[[ ${EUID:-$(id -u)} -eq 0 ]] || die "This script must be run as root. Use: curl ... | sudo bash"

need curl
need systemctl

# ── detect architecture ──────────────────────────────────────────────────────
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)          GOARCH="amd64" ;;
  aarch64|arm64)   GOARCH="arm64" ;;
  *) die "Unsupported architecture: ${ARCH}. Only amd64 and arm64 are supported." ;;
esac
info "Detected architecture: ${ARCH} (${GOARCH})"

# ── resolve latest release ───────────────────────────────────────────────────
info "Fetching latest release info from GitHub..."
API_RESPONSE="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")"

# Extract download URL for the matching binary asset
DOWNLOAD_URL="$(
  printf '%s' "${API_RESPONSE}" \
    | grep '"browser_download_url"' \
    | grep "mailhamster-linux-${GOARCH}" \
    | head -1 \
    | sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/'
)"

[[ -n "${DOWNLOAD_URL}" ]] || die \
  "Could not find a release binary for linux-${GOARCH}. \
Check https://github.com/${REPO}/releases for available assets."

VERSION="$(
  printf '%s' "${API_RESPONSE}" \
    | grep '"tag_name"' \
    | head -1 \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/'
)"
info "Installing MailHamster ${VERSION} (linux-${GOARCH})"

# ── download and install binary ──────────────────────────────────────────────
TMP="$(mktemp)"
trap 'rm -f "${TMP}"' EXIT

curl -fsSL --progress-bar "${DOWNLOAD_URL}" -o "${TMP}"
chmod 0755 "${TMP}"
mv "${TMP}" "${INSTALL_BIN}"
info "Binary installed: ${INSTALL_BIN}"

# ── create system user ───────────────────────────────────────────────────────
if ! id "${SERVICE_USER}" &>/dev/null; then
  info "Creating system user: ${SERVICE_USER}"
  useradd --system --no-create-home --shell /usr/sbin/nologin "${SERVICE_USER}"
else
  info "System user already exists: ${SERVICE_USER}"
fi

# ── install configuration ─────────────────────────────────────────────────────
mkdir -p "${CONF_DIR}"

if [[ ! -f "${CONF_FILE}" ]]; then
  info "Writing example config: ${CONF_FILE}"
  cat > "${CONF_FILE}" << 'YAML'
# MailHamster configuration — edit this file before starting the service.
# Full reference: https://github.com/Tiboriuss/MailHamster

listen:
  addr: "127.0.0.1:25"

auth:
  users:
    - username: "myapp"
      password: "changeme"

upstream:
  host: "smtp.example.com"
  port: 587
  username: ""
  password: ""
  # tls: none | starttls | tls
  tls: "starttls"

rewrite:
  enabled: false
  from: "noreply@example.com"
  from_name: "My Application"

logging:
  level: "info"
  format: "text"
YAML
  chmod 640 "${CONF_FILE}"
  chown "root:${SERVICE_USER}" "${CONF_FILE}"
else
  info "Config already exists, not overwriting: ${CONF_FILE}"
fi

# ── install systemd unit ──────────────────────────────────────────────────────
info "Installing systemd unit: ${SERVICE_FILE}"
cat > "${SERVICE_FILE}" << 'UNIT'
[Unit]
Description=MailHamster SMTP Relay
Documentation=https://github.com/Tiboriuss/MailHamster
After=network.target

[Service]
Type=simple
User=mailhamster
Group=mailhamster
ExecStart=/usr/local/bin/mailhamster --config /etc/mailhamster/mailhamster.yaml
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mailhamster
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictRealtime=true
MemoryDenyWriteExecute=true
SystemCallFilter=@system-service
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
ReadOnlyPaths=/
ReadWritePaths=/etc/mailhamster

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"

# ── done ─────────────────────────────────────────────────────────────────────
echo ""
echo "  MailHamster ${VERSION} installed successfully!"
echo ""
echo "  Next steps:"
echo "    1. Edit your config:  ${CONF_FILE}"
echo "       (set upstream.host, upstream.username, upstream.password, and auth.users)"
echo ""
echo "    2. Start the service: systemctl start ${SERVICE_NAME}"
echo "    3. Check status:      systemctl status ${SERVICE_NAME}"
echo "    4. Follow logs:       journalctl -u ${SERVICE_NAME} -f"
echo ""
echo "  To uninstall:"
echo "    systemctl disable --now ${SERVICE_NAME}"
echo "    rm -f ${INSTALL_BIN} ${SERVICE_FILE}"
echo "    rm -rf ${CONF_DIR}"
echo "    userdel ${SERVICE_USER}"
echo ""
