#!/usr/bin/env bash
# MailHamster offline installer
# Place this script in the same directory as the mailhamster binary, then run:
#   sudo bash offline-install.sh
set -euo pipefail

INSTALL_BIN="/usr/local/bin/mailhamster"
CONF_DIR="/etc/mailhamster"
CONF_FILE="${CONF_DIR}/mailhamster.yaml"
SERVICE_FILE="/etc/systemd/system/mailhamster.service"
SERVICE_NAME="mailhamster"

# ── helpers ───────────────────────────────────────────────────────────────────
die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }

# ── pre-flight checks ─────────────────────────────────────────────────────────
[[ ${EUID:-$(id -u)} -eq 0 ]] || die "This script must be run as root."

command -v systemctl &>/dev/null || die "systemd is required but systemctl was not found."

# Resolve the directory this script lives in, following symlinks
SCRIPT_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
BINARY="${SCRIPT_DIR}/mailhamster"

[[ -f "${BINARY}" ]] || die "Binary not found: ${BINARY}
Make sure the mailhamster binary is in the same directory as this script."

[[ -x "${BINARY}" ]] || chmod +x "${BINARY}"

# ── install binary ────────────────────────────────────────────────────────────
info "Installing binary: ${INSTALL_BIN}"
cp "${BINARY}" "${INSTALL_BIN}"
chmod 0755 "${INSTALL_BIN}"

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
  chmod 600 "${CONF_FILE}"
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
ExecStart=/usr/local/bin/mailhamster --config /etc/mailhamster/mailhamster.yaml
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mailhamster

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"

# ── done ──────────────────────────────────────────────────────────────────────
echo ""
echo "  MailHamster installed successfully!"
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
echo "    systemctl daemon-reload"
echo ""
