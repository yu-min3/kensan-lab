# Raspberry Pi WiFi 安定化設定

Raspberry Pi 5 をKubernetesノードとしてWiFi（wlan0）で運用する際の安定化設定。

> **推奨**: 可能であれば有線LAN（eth0）を使用してください。WiFiはKubernetesノードの接続手段としては本質的に不安定です。

## 背景・課題

ベアメタルクラスター（RPi5 x3）を WiFi のみで運用した場合、以下の問題が発生する:

- **WiFi power save** によりアダプタがスリープ → ノードが NotReady になる
- **brcmfmac ドライバのローミング** で不要な再接続が発生し一時的に断線する
- **DHCP リース切れ** で IP アドレスが変動・喪失する
- 上記が発生すると SSH も含めて一切到達不能になり、**物理再起動が必要**

## ノード構成

| ノード | 静的IP | WiFi SSID | watchdog ping先 |
|--------|--------|-----------|-----------------|
| master | 192.168.0.XXX/24 | YOUR_SSID | 192.168.0.1 (gateway) |
| worker1 | 192.168.0.XXX/24 | YOUR_SSID | 192.168.0.XXX (master) |
| worker2 | 192.168.0.XXX/24 | YOUR_SSID | 192.168.0.XXX (master) |

## 対策（4点セット）

### 1. WiFi Power Save 無効化

brcmfmac ドライバはデフォルトで power save が有効。`dmesg` で確認可能:

```
brcmf_cfg80211_set_power_mgmt: power save enabled
```

**即時無効化:**

```bash
sudo iw wlan0 set power_save off
```

**永続化（systemd service）:**

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

> **注意**: `iw` パッケージが必要（`sudo apt-get install iw`）

### 2. brcmfmac Roaming 無効化

WiFi ドライバが別の AP を探してローミングしようとする動作を抑止する。

`/etc/modprobe.d/brcmfmac.conf`:

```
options brcmfmac roamoff=1
```

> このパラメータは sysfs が read-only のため、適用にはノードの再起動が必要。

### 3. 静的IP固定（netplan）

DHCP リース切れによる IP 喪失を防止する。

`/etc/netplan/50-cloud-init.yaml`:

```yaml
network:
  version: 2
  wifis:
    wlan0:
      optional: true
      dhcp4: false
      addresses:
        - 192.168.0.XXX/24       # ノードごとに変更
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

> **重要**: ルーター側で該当 IP を DHCP 除外するか、MAC アドレス予約を行うこと。

### 4. WiFi Watchdog（自動復旧）

30秒間隔で ping を実行し、3回連続失敗で wlan0 を自動再起動する。

**watchdog スクリプト** (`/usr/local/bin/wifi-watchdog.sh`):

```bash
#!/bin/bash
PING_TARGET="192.168.0.107"  # master→gateway(192.168.0.1) に変更
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

        # wlan0 再起動でもダメなら brcmfmac リロード
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

## セットアップスクリプト

全ノードへの一括適用スクリプトが `temp/fix-wifi.sh` にある。

```bash
# 各ノードへの適用
scp temp/fix-wifi.sh your-user@<host>:~/
ssh your-user@<host> "sudo bash ~/fix-wifi.sh <STATIC_IP/CIDR> <PING_TARGET>"

# 例
# worker2: sudo bash fix-wifi.sh 192.168.0.109/24 192.168.0.107
# worker1: sudo bash fix-wifi.sh 192.168.0.108/24 192.168.0.107
# master:  sudo bash fix-wifi.sh 192.168.0.107/24 192.168.0.1
```

## 確認コマンド

```bash
# Power save 状態
iw wlan0 get power_save

# Roaming 無効化（再起動後）
cat /sys/module/brcmfmac/parameters/roamoff

# 割り当てIP
ip addr show wlan0

# Watchdog 稼働状況
systemctl status wifi-watchdog.timer

# Watchdog ログ（リアルタイム）
journalctl -t wifi-watchdog -f
```

## トラブルシューティング

### ノードに SSH できない
1. IP 直指定で試す: `ssh your-user@192.168.0.XXX`（mDNS が一時的に切れている場合）
2. 物理アクセスで再起動（watchdog が間に合わないケースでは必要）

### netplan apply 後に接続が切れた
- IP を変更した場合は当然切れる。新しい IP で再接続する
- IP を変えていないのに切れた場合は数秒待って再接続（WiFi の再アソシエーション中）

### apt-get update が失敗する
- CRI-O リポジトリ (`pkgs.k8s.io`) が 403 を返すことがある
- `iw` のインストールは Ubuntu 公式リポジトリから取得するため、他リポジトリのエラーは無視して良い

## 設定日

- 初回適用: 2026-02-18
- 対象: master, worker1, worker2（全ノード）
