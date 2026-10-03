---
title: sense v0 bounded publisher
status: active
---

# 限定 publisher

`sense-dev-publisher` は controller / model worker とは別の手動実行プロセス。`branch_push` と `pr_create` のみを扱う。対象は `yu-min3/kensan-lab` の `main` 以外の branch と、`main` 宛て draft PR に固定する。GitHub 操作は **実モデルの独立 Release Gate が作成した、期限内・同一 SHA の allow decision** があるときだけ可能。模擬 session の判定からは `PreparePublish` が失敗する。

## 手順

1. Gate の decision ID、repo/ref/head SHA、差分・scan、CI/公開/配備影響、rollback を確認する。
2. publisher 専用プロセスに、対象 repo のみへアクセスする GitHub token を mode `0600` の file で渡す。model worker の auth home、controller token、一般ユーザーの Git credential は渡さない。branch push には token を読む専用 `GIT_ASKPASS` executable を指定する。
3. `sense-dev-publisher -data <controller-state> -decision <id> -repo <trusted-checkout> -token-file <private-file> -askpass <trusted-executable>` を実行する。PR の場合、先に別 decision で同一 SHA の branch push を終える。
4. publisher は remote branch / PR を先に照合する。既存の同一 SHA なら送信済みとして台帳に記録する。異なる SHA は拒否する。初回は intent を `sending` に固定してから外部操作する。
5. 失敗・プロセス停止で結果が曖昧な場合は `unknown` として自動再試行しない。remote を照合し、無ければ新しい Gate 判定と人手で再実行を決める。

この CLI は timer / UI から自動起動しない。host 上の publisher 用 credential は未配置。既存 mock-only service にも publisher は接続されない。GitHub の実 push / PR は独立 Gate と private-ready の実証後に行い、操作結果を T027 の証拠に残す。

## 境界

- push は trusted checkout に approved commit が実在し、`origin` が固定 HTTPS repo と一致したときだけ。`git` の端末 prompt、system/global config、credential helper を無効化する。
- PR は approved branch の remote SHA が一致するときだけ作る。Gate が固定した summary を本文に使い、draft として作る。
- token は command line と event に入れない。外部 API エラー本文もログへ出さない。
- `merge`、`deploy`、`rollback`、PR 更新は v0 publisher の許可範囲外。別の実装と判定が必要。
