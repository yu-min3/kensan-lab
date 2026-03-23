# Incident: system-infra ArgoCD 自動 sync ブロック

## 概要

| 項目 | 内容 |
|------|------|
| 発生日 | 2026-03-02 |
| 検知日 | 2026-03-23 |
| 影響 | kube-system namespace のラベル更新が約3週間反映されず、hubble-route の HTTPRoute が Gateway に拒否され Cilium アプリが Degraded |
| 根本原因 | ArgoCD が kube-system namespace を prune しようとし DeletionError でブロック |

## タイムライン

| 日時 | イベント |
|------|---------|
| 2025-11-17 | system-infra 最後の成功 sync（ディレクトリ再配置） |
| 2026-02-21 | PSS baseline ラベル追加（system-infra 最後のファイル変更） |
| 2026-03-02 11:37 UTC | DeletionError 発生: `namespaces "kube-system" is forbidden: this namespace may not be deleted` |
| 2026-03-19 | goldship → kensan-lab ラベルリネームがコミットされるも sync されず |
| 2026-03-23 | Backstage 調査中に Cilium Degraded を検知、原因調査で発覚 |

## 影響の連鎖

```
DeletionError (kube-system prune 失敗)
  → system-infra の自動 sync が全停止
    → goldship.platform/* ラベルが kube-system に残留
      → Gateway の allowedRoutes selector (kensan-lab.platform/*) にマッチせず
        → hubble-route HTTPRoute が NotAllowedByListeners
          → ArgoCD が Cilium アプリを Degraded と判定
```

## 根本原因の分析

### 直接原因

ArgoCD の ApplicationSet generator（environments）が一時的に system-infra の config.json 読み取りに失敗したか、repo-server のキャッシュ不整合により、kube-system namespace が管理対象から外れたと推測される。`prune: true` 設定により削除を試行したが、Kubernetes API が kube-system の削除を拒否し DeletionError が発生。

### なぜ長期間検知できなかったか

- system-infra は namespace ラベルのみを管理しており、ラベルの差分だけでは実害が見えにくい
- DeletionError は ArgoCD UI の conditions に表示されるが、アプリ一覧では OutOfSync 表示のみ
- Cilium の Degraded は hubble-route（監視 UI）の問題であり、CNI 自体は正常動作していた

### なぜ 3/2 にコミットなしで発生したか

3/2 にはリポジトリへのコミットがない。ArgoCD の定期 reconciliation（デフォルト3分間隔）で ApplicationSet generator が Git をスキャンした際の一時的な不整合が原因と推測される。repo-server の再起動やネットワーク瞬断なども可能性としてあるが、ログが残っておらず特定不能。

## 対応

1. `kubectl annotate application system-infra argocd.argoproj.io/refresh=hard` で手動 refresh
2. prune=false で手動 sync トリガー → Synced に復帰
3. kube-system の namespace.yaml に `argocd.argoproj.io/sync-options: Prune=false` アノテーションを追加（再発防止）

## 再発防止

- kube-system 等の Kubernetes 保護 namespace には `Prune=false` sync option を付与する
- ArgoCD で管理するが削除されてはならないリソースには防御的にアノテーションを設定する
