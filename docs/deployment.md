# Deployment

AprsMonitor is one binary plus one config file. It makes only outbound
connections (APRS-IS, SMTP, HTTPS), so it needs no open ports.

**Where to run it.** On a machine that is *not* your iGate, with a different
internet connection if you can. A monitor running on the iGate cannot tell you
the machine lost power or its network. A small VPS, a home server, or a
Raspberry Pi elsewhere all work.

## Build

```
go build -ldflags "-X main.version=0.1.0" -o aprsmonitor ./cmd/aprsmonitor
```

Cross-compile by setting `GOOS`/`GOARCH`, for example `GOOS=linux GOARCH=arm64`
for a 64-bit Raspberry Pi, or `GOOS=windows GOARCH=amd64`.

## Linux (systemd)

```
sudo useradd --system --no-create-home aprsmonitor
sudo install -m 755 aprsmonitor /usr/local/bin/aprsmonitor
sudo install -d -m 750 -o root -g aprsmonitor /etc/aprsmonitor
sudo install -m 640 -o root -g aprsmonitor config.toml /etc/aprsmonitor/config.toml
```

`/etc/systemd/system/aprsmonitor.service`:

```ini
[Unit]
Description=APRS iGate monitor
After=network-online.target
Wants=network-online.target

[Service]
User=aprsmonitor
ExecStart=/usr/local/bin/aprsmonitor -config /etc/aprsmonitor/config.toml
# Optional: secrets referenced as env:NAME in the config
# EnvironmentFile=/etc/aprsmonitor/secrets.env
Restart=always
RestartSec=10
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```
sudo systemctl daemon-reload
sudo systemctl enable --now aprsmonitor
journalctl -u aprsmonitor -f
```

## Windows

Easiest is Task Scheduler (no extra software):

1. Put `aprsmonitor.exe` and `config.toml` in `C:\aprsmonitor`.
2. Create a task: trigger *At startup*, action *Start a program*
   `C:\aprsmonitor\aprsmonitor.exe`, *Start in* `C:\aprsmonitor`.
3. In Settings, tick *If the task fails, restart every 1 minute*, and untick
   *Stop the task if it runs longer than*.

To run as a real Windows service, wrap the exe with
[NSSM](https://nssm.cc/) (`nssm install AprsMonitor C:\aprsmonitor\aprsmonitor.exe`).

## Docker

```dockerfile
FROM golang:1.27 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /aprsmonitor ./cmd/aprsmonitor

FROM gcr.io/distroless/static
COPY --from=build /aprsmonitor /aprsmonitor
ENTRYPOINT ["/aprsmonitor", "-config", "/config/config.toml"]
```

```
docker run -d --restart unless-stopped -v "$PWD/config.toml:/config/config.toml:ro" aprsmonitor
```

## After installing

1. `aprsmonitor -check-config` must print `config OK`.
2. `aprsmonitor -test-alert` must deliver a message to every channel you set up.
3. Start the service. Logs show `aprs-is connected` within seconds.
4. Prove the whole chain once: stop your iGate for longer than
   `silence_threshold` and confirm the alert arrives, then start it and confirm
   the recovery message.
