# Syncthing ペアリング復旧 runbook

kensan-workspace のローカルファースト同期（Mac ⇄ クラスタ、PR #371）の
config PVC（`app-kensan/syncthing-config`）が失われた場合の再構築手順。
device 鍵・index DB はこの PVC にのみ存在し、Git には載らない（意図的な tradeoff）。

## 失われるもの / 失われないもの

- 失われる: cluster 側 device 鍵（= Mac とのペアリング）、folder 設定、GUI 設定
- 失われない: **workspace データ本体**（別 PVC `kensan-workspace`）と Mac 側の完全コピー

## 再構築手順（約 10 分）

1. pod 再作成で新しい device 鍵が生成される。新 ID を取得:
   `kubectl logs -n app-kensan deploy/syncthing | grep "My ID"`
2. cluster 側の folder / LAN 限定設定（apikey は config.xml から取得して cli に渡す）:
   ```
   kubectl exec -n app-kensan deploy/syncthing -- sh -c '
   KEY=$(sed -n "s/.*<apikey>\(.*\)<\/apikey>.*/\1/p" /var/syncthing/config/config.xml)
   stcli() { syncthing cli --gui-address=127.0.0.1:8384 --gui-apikey="$KEY" "$@"; }
   stcli config folders add --id kensan-workspace --label kensan-workspace --path /var/syncthing/data
   stcli config options global-ann-enabled set false
   stcli config options relays-enabled set false
   stcli config options natenabled set false
   stcli config devices add --device-id <MacのID> --name mac
   stcli config folders kensan-workspace devices add --device-id <MacのID>'
   ```
3. Mac 側で旧 cluster device を削除し、新 ID を登録:
   ```
   syncthing cli config devices add --device-id <新ID> --name cluster --addresses tcp://192.168.0.245:22000
   syncthing cli config folders kensan-workspace devices add --device-id <新ID>
   ```
4. 再スキャン後、両側の内容はほぼ同一なので転送は差分のみ。conflict が出たら内容確認して解消

## 関連の運用メモ

- GUI(8384) は NetworkPolicy `syncthing-guard` でクラスタ内からも遮断。設定操作は kubectl exec のみ
- .stignore は同期されない設定。両側で `/repositories` 等の内容を揃えること（kensan-workspace の conventions.md 参照）
- git 操作は Mac のみ（cluster 側で .git を書かない規律）
- Mac 側 Syncthing: brew services（`brew services restart syncthing`）
