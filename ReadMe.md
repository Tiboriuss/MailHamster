# MailHamster

MailHamster is a small mail relay service for Linux servers. You configure it once with your real SMTP credentials (e.g. Gmail, Mailgun, your own mail server), and any application on the same machine can send mail through it on port 25 — no TLS setup, no per-app SMTP configuration.

**Typical use case:** You have a web app, a script, or a monitoring tool that needs to send email. Instead of putting SMTP credentials in every app, you install MailHamster once and point everything at `localhost:25`.

---

## Requirements

- A Linux server with **systemd**
- Root access
- An outgoing SMTP server to relay through (e.g. Gmail, Mailgun, Sendgrid, your own)

---

## Installation

### Online (recommended)

Run this as root on your server:

```bash
curl -fsSL https://raw.githubusercontent.com/Tiboriuss/MailHamster/main/install.sh | sudo bash
```

This downloads the correct binary for your architecture, creates the config file at `/etc/mailhamster/mailhamster.yaml`, and registers it as a system service. The service is **not started automatically** — you need to edit the config first.

### Offline

If your server has no internet access, download two files from the [Releases page](https://github.com/Tiboriuss/MailHamster/releases/latest) onto your local machine first:

- `mailhamster-linux-amd64` (or `mailhamster-linux-arm64` for ARM servers)
- `offline-install.sh`

Copy both to the server (e.g. via `scp`), rename the binary to `mailhamster`, and run the installer:

```bash
scp mailhamster-linux-amd64 offline-install.sh root@yourserver:/tmp/
ssh root@yourserver
cd /tmp
mv mailhamster-linux-amd64 mailhamster
bash offline-install.sh
```

The script expects the `mailhamster` binary to be in the same directory as `offline-install.sh`.

---

## Setup

Open the config file:

```bash
nano /etc/mailhamster/mailhamster.yaml
```

There are three things you need to fill in:

**1. Your outgoing mail server** (`upstream` section)

This is the SMTP server MailHamster will forward mail to. Use the credentials from your mail provider.

| Provider | Host | Port | TLS |
|---|---|---|---|
| Gmail | `smtp.gmail.com` | 587 | `starttls` |
| Mailgun | `smtp.mailgun.org` | 587 | `starttls` |
| Sendgrid | `smtp.sendgrid.net` | 587 | `starttls` |
| Office 365 | `smtp.office365.com` | 587 | `starttls` |
| Custom (TLS) | your host | 465 | `tls` |
| Custom (plain) | your host | 25 | `none` |

**2. Local credentials** (`auth` section)

Pick a username and password that your applications will use to connect to MailHamster on `localhost:25`. These are separate from your upstream credentials — make them whatever you like.

**3. From address rewriting** (`rewrite` section, optional)

If your upstream provider only allows mail from a specific sender address (common with shared relays), enable rewriting and set that address here.

### Example config

```yaml
listen:
  addr: "127.0.0.1:25"

auth:
  users:
    - username: "myapp"
      password: "a-strong-password"

upstream:
  host: "smtp.mailgun.org"
  port: 587
  username: "postmaster@mg.example.com"
  password: "your-mailgun-smtp-password"
  tls: "starttls"

rewrite:
  enabled: false          # set to true if your upstream requires a fixed sender
  from: "noreply@example.com"
  from_name: "My Server"

logging:
  level: "info"           # use "debug" to see detailed relay logs
  format: "text"
```

---

## Starting the service

Once the config is saved:

```bash
systemctl start mailhamster
```

Check that it is running:

```bash
systemctl status mailhamster
```

View live logs:

```bash
journalctl -u mailhamster -f
```

The service starts automatically on boot.

---

## Sending mail through MailHamster

From any application on the same server, point your SMTP settings at:

- **Host:** `127.0.0.1`
- **Port:** `25`
- **Username / Password:** whatever you set in `auth.users`
- **TLS:** none (the connection stays on localhost)

---

## Troubleshooting

**451 relay failed** — MailHamster accepted the message but could not deliver it to the upstream server. Check your upstream credentials and host/port in the config. Run `journalctl -u mailhamster -f` and try again to see the exact error.

**535 authentication credentials invalid** — The username or password your application used does not match anything in `auth.users`.

**Connection refused on port 25** — The service is not running. Run `systemctl start mailhamster` and check `systemctl status mailhamster` for errors.

---

## Uninstalling

```bash
systemctl disable --now mailhamster
rm -f /usr/local/bin/mailhamster /etc/systemd/system/mailhamster.service
rm -rf /etc/mailhamster
systemctl daemon-reload
```