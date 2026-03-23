# MailHamster

A lightweight SMTP relay daemon for Linux. It listens on localhost with username/password authentication, optionally rewrites the `From` header, and forwards mail to a configured upstream server. Ships as a single static binary.

## How it works

1. Your application connects to `127.0.0.1:25` and authenticates with PLAIN credentials defined in the config
2. MailHamster (optionally) rewrites the `From` header
3. The message is relayed to the configured upstream SMTP server using plain, STARTTLS, or implicit TLS

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/Tiboriuss/MailHamster/main/install.sh | sudo bash
```

The installer will:
- Download the correct binary for your architecture (amd64 / arm64)
- Install an example config at `/etc/mailhamster/mailhamster.yaml`
- Install and enable the systemd service (does **not** start it automatically — edit the config first)

**Requirements:** systemd, curl, root access.

## Configuration

After installation, edit `/etc/mailhamster/mailhamster.yaml`:

```yaml
listen:
  addr: "127.0.0.1:25"

auth:
  users:
    - username: "myapp"
      password: "changeme"

upstream:
  host: "smtp.example.com"
  port: 587
  username: "relay@example.com"
  password: "upstreampassword"
  # tls: none | starttls | tls
  tls: "starttls"

rewrite:
  enabled: false
  from: "noreply@example.com"
  from_name: "My Application"

logging:
  level: "info"    # debug | info | warn | error
  format: "text"   # text | json
```

Then start the service:

```bash
systemctl start mailhamster
systemctl status mailhamster
journalctl -u mailhamster -f
```

### Upstream TLS modes

| Mode | Description | Typical port |
|---|---|---|
| `none` | Plain SMTP, no encryption | 25 |
| `starttls` | Upgrades to TLS via STARTTLS | 587 |
| `tls` | Implicit TLS from the start | 465 |

### From rewriting

When `rewrite.enabled: true`, MailHamster replaces the `From` (and `Sender` if present) header before relaying. This is useful when your upstream server only permits a single sender address (e.g. a shared transactional relay).

## Building from source

Requires Go 1.22+.

```bash
git clone https://github.com/Tiboriuss/MailHamster.git
cd MailHamster
make build          # produces bin/mailhamster (native)
make release        # cross-compiles dist/mailhamster-linux-{amd64,arm64}
```

## Releasing

Releases are automated via GitHub Actions and GoReleaser. Push a version tag to trigger a build:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The action compiles Linux binaries for amd64 and arm64, generates a SHA-256 checksum file, and publishes them as a GitHub Release.

## Security notes

- The listener has no TLS — it is intentionally localhost-only. Do not expose it on a network-facing interface.
- The service runs as root, which allows it to bind to port 25 without any additional configuration.
- The config file should only be readable by root: `chmod 600 /etc/mailhamster/mailhamster.yaml` (the installer sets this automatically).

## Uninstalling

```bash
systemctl disable --now mailhamster
rm -f /usr/local/bin/mailhamster /etc/systemd/system/mailhamster.service
rm -rf /etc/mailhamster
systemctl daemon-reload
```