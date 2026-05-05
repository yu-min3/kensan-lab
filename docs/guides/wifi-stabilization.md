# Raspberry Pi WiFi Stabilization Settings

Stabilization settings for running Raspberry Pi 5 as Kubernetes nodes over WiFi (wlan0).

> **Recommendation**: Use wired LAN (eth0) if possible. WiFi is inherently unstable as a connectivity method for Kubernetes nodes.

## Background and Issues

When operating a bare-metal cluster (RPi5 x3) on WiFi only, the following problems occur:

- **WiFi power save** puts the adapter to sleep -> Node becomes NotReady
- **brcmfmac driver roaming** causes unnecessary reconnections resulting in temporary disconnections
- **DHCP lease expiration** causes IP address changes or loss
- When any of the above occurs, the node becomes completely unreachable including SSH, **requiring a physical restart**

## Node Configuration

| Node | Static IP | WiFi SSID | Watchdog Ping Target |
|--------|--------|-----------|-----------------|
| master | 192.168.0.XXX/24 | YOUR_SSID | 192.168.0.1 (gateway) |
| worker1 | 192.168.0.XXX/24 | YOUR_SSID | 192.168.0.XXX (master) |
| worker2 | 192.168.0.XXX/24 | YOUR_SSID | 192.168.0.XXX (master) |

## Countermeasures (4-Part Solution)

### 1. Disable WiFi Power Save

The brcmfmac driver has power save enabled by default. Verifiable in `dmesg`:

```
brcmf_cfg80211_set_power_mgmt: power save enabled
```

**Immediate disable:**

```bash
sudo iw wlan0 set power_save off
```

**Persist (systemd service):**

`/etc/systemd/system/wifi-powersave-off.service`:

```ini
[Unit]
Description=Disable WiFi Power Save
After=sys-subsystem-net-devices-wlan0.device
Wants=sys-subsystem-net-devices-wlan0.device

[Service]
Type=oneshot
ExecStart=/sbin/iw wlan0 set power_save off
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable wifi-powersave-off.service
```

> **Note**: The `iw` package is required (`sudo apt-get install iw`)

### 2. Disable brcmfmac Roaming

Prevent the WiFi driver from attempting to roam to other APs.

`/etc/modprobe.d/brcmfmac.conf`:

```
options brcmfmac roamoff=1
```

> This parameter requires a node reboot to take effect since sysfs is read-only.

### 3. Static IP (netplan)

Prevent IP loss from DHCP lease expiration.

`/etc/netplan/50-cloud-init.yaml`:

```yaml
network:
  version: 2
  wifis:
    wlan0:
      optional: true
      dhcp4: false
      addresses:
        - 192.168.0.XXX/24       # Change per node
      routes:
        - to: default
          via: 192.168.0.1
      nameservers:
        addresses: [192.168.0.1, 8.8.8.8]
      access-points:
        "YOUR_SSID":
          auth:
            key-management: "psk"
            password: "<hashed-psk>"
```

```bash
sudo netplan apply
```

> **Important**: Exclude the relevant IPs from DHCP on the router side, or set up MAC address reservations.

### 4. WiFi Watchdog (Automatic Recovery)

Pings every 30 seconds and automatically restarts wlan0 after 3 consecutive failures.

**Watchdog script** (`/usr/local/bin/wifi-watchdog.sh`):

```bash
#!/bin/bash
PING_TARGET="192.168.0.107"  # For master, change to gateway (192.168.0.1)
FAIL_THRESHOLD=3
FAIL_COUNT_FILE="/tmp/wifi-watchdog-fail-count"

FAIL_COUNT=$(cat "$FAIL_COUNT_FILE" 2>/dev/null || echo 0)

if ping -c 1 -W 5 "$PING_TARGET" > /dev/null 2>&1; then
    echo 0 > "$FAIL_COUNT_FILE"
else
    FAIL_COUNT=$((FAIL_COUNT + 1))
    echo "$FAIL_COUNT" > "$FAIL_COUNT_FILE"
    logger -t wifi-watchdog "ping to $PING_TARGET failed ($FAIL_COUNT/$FAIL_THRESHOLD)"

    if [ "$FAIL_COUNT" -ge "$FAIL_THRESHOLD" ]; then
        logger -t wifi-watchdog "threshold reached, restarting wlan0"
        ip link set wlan0 down
        sleep 2
        ip link set wlan0 up
        sleep 5
        iw wlan0 set power_save off
        echo 0 > "$FAIL_COUNT_FILE"

        # If wlan0 restart doesn't help, reload brcmfmac
        if ! ping -c 1 -W 10 "$PING_TARGET" > /dev/null 2>&1; then
            logger -t wifi-watchdog "wlan0 restart failed, reloading brcmfmac"
            modprobe -r brcmfmac
            sleep 3
            modprobe brcmfmac
            sleep 10
            iw wlan0 set power_save off
        fi
    fi
fi
```

**systemd service** (`/etc/systemd/system/wifi-watchdog.service`):

```ini
[Unit]
Description=WiFi Watchdog - Check connectivity and recover
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/wifi-watchdog.sh
```

**systemd timer** (`/etc/systemd/system/wifi-watchdog.timer`):

```ini
[Unit]
Description=WiFi Watchdog Timer

[Timer]
OnBootSec=60
OnUnitActiveSec=30
AccuracySec=5

[Install]
WantedBy=timers.target
```

```bash
sudo chmod +x /usr/local/bin/wifi-watchdog.sh
sudo systemctl daemon-reload
sudo systemctl enable --now wifi-watchdog.timer
```

## Setup Script

A batch application script for all nodes is available at `temp/fix-wifi.sh`.

```bash
# Apply to each node
scp temp/fix-wifi.sh your-user@<host>:~/
ssh your-user@<host> "sudo bash ~/fix-wifi.sh <STATIC_IP/CIDR> <PING_TARGET>"

# Examples
# worker2: sudo bash fix-wifi.sh 192.168.0.109/24 192.168.0.107
# worker1: sudo bash fix-wifi.sh 192.168.0.108/24 192.168.0.107
# master:  sudo bash fix-wifi.sh 192.168.0.107/24 192.168.0.1
```

## Verification Commands

```bash
# Power save status
iw wlan0 get power_save

# Roaming disabled (after reboot)
cat /sys/module/brcmfmac/parameters/roamoff

# Assigned IP
ip addr show wlan0

# Watchdog operational status
systemctl status wifi-watchdog.timer

# Watchdog logs (real-time)
journalctl -t wifi-watchdog -f
```

## Troubleshooting

### Cannot SSH into Node
1. Try with direct IP: `ssh your-user@192.168.0.XXX` (mDNS may be temporarily down)
2. Physical access and restart (necessary when the watchdog can't recover in time)

### Connection Lost After netplan apply
- If you changed the IP, disconnection is expected. Reconnect using the new IP
- If you didn't change the IP and still disconnected, wait a few seconds and reconnect (WiFi re-association in progress)

### apt-get update Fails
- CRI-O repository (`pkgs.k8s.io`) may return 403
- `iw` is installed from the official Ubuntu repository, so errors from other repositories can be ignored

## Date

- Initial application: 2026-02-18
- Target: master, worker1, worker2 (all nodes)
