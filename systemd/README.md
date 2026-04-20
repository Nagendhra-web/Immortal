# Systemd service for Immortal

Run Immortal as a long-lived Linux service with automatic restart, resource limits, and reasonable security hardening.

## Install

```sh
# 1. Install the binary (pick one)
curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | bash
# or
go install github.com/Nagendhra-web/Immortal/cmd/immortal@latest
# move to /usr/local/bin if installed elsewhere
sudo install -m 0755 "$(go env GOBIN)/immortal" /usr/local/bin/immortal

# 2. Create the immortal user + data dir
sudo useradd --system --no-create-home --shell /usr/sbin/nologin immortal
sudo install -d -o immortal -g immortal -m 0750 /var/lib/immortal

# 3. Install the unit file
sudo install -m 0644 systemd/immortal.service /etc/systemd/system/immortal.service

# 4. Optional environment overrides
sudo install -d -m 0750 -o root -g immortal /etc/immortal
sudo sh -c 'cat > /etc/immortal/env <<EOF
IMMORTAL_LOG_LEVEL=info
# OPENROUTER_API_KEY=sk-or-...
EOF'
sudo chmod 0640 /etc/immortal/env

# 5. Start + enable on boot
sudo systemctl daemon-reload
sudo systemctl enable --now immortal.service

# 6. Verify
systemctl status immortal.service
journalctl -u immortal.service -f
curl -sf http://127.0.0.1:7777/api/health
```

## Upgrade

```sh
sudo systemctl stop immortal.service
curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | IMMORTAL_INSTALL=/usr/local/bin bash
sudo systemctl start immortal.service
```

## Uninstall

```sh
sudo systemctl disable --now immortal.service
sudo rm /etc/systemd/system/immortal.service
sudo systemctl daemon-reload
sudo rm -rf /var/lib/immortal /etc/immortal /usr/local/bin/immortal
sudo userdel immortal
```

## Hardening notes

The shipped unit file already enables:

- `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`
- `PrivateTmp`, `LockPersonality`
- `SystemCallFilter=@system-service`
- Resource caps: `LimitNOFILE=65536`, `MemoryMax=2G`

For stricter environments (government / regulated), add:

```ini
CapabilityBoundingSet=
AmbientCapabilities=
ProtectHostname=true
ProtectClock=true
```

## Troubleshooting

- **`status=203/EXEC`**: binary path wrong, edit the unit file and reload.
- **Permission denied on `/var/lib/immortal`**: user mismatch. Check that step 2 above ran.
- **Engine exits with code 2**: audit chain verification failed. Check `journalctl -u immortal -n 200`.
