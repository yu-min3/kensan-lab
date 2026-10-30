# 常設表示の成果・開発指標

常設画面は、保存済みの公開・開発観測と既存project metricsを読む。画面のGETやpollで外部API取得・推論・保存を起こさない。

| 指標 | 定義・正本 |
|---|---|
| 今月公開した記事 | Zennの`published_at`をJST暦月で集計。下書き・既存記事の編集を除外。対象媒体を画面に明記 |
| kensan-lab Stars | 既存の`projects/kensan-lab/metrics.yaml`・`metrics.ndjson`。取得時刻・既存の30日差分を表示 |
| 今週のマージPR | 設定した公開GitHub repoの`merged_at`をJST月曜0時〜現在で集計。却下・未マージ・未来日時を除外 |
| マージまでの中央値 | 同じPRの`merged_at - created_at`。作業時間・配備リードタイムとしては扱わない |

## 収集と保存

`cmd/watch-metrics`は`-repo owner/repo -zenn username`で対象を受け取り、`KENSAN_DATA_DIR`配下の`feeds/watch-metrics/latest.json`へ公開APIの観測を保存する。非公開repoやAPI資格は対象外。GitHubの[PR一覧API](https://docs.github.com/en/rest/pulls/pulls#list-pull-requests)は更新順にページングし、週境界より古い更新まで確認する。Zennはサイトの公開一覧JSONを最終ページまで確認する（`total_count`はnullになり得る）。エラーや不完全な応答・ページ上限を0件として確定しない。

公開記事と開発指標は独立して前回成功値・成功時刻を保持し、最終試行と失敗を分けて保存する。期間が変わった古い値は当期のカードへ流用せず「—」。同期間の3時間超の値・取得失敗時の値は前回値と表示する。排他は`feeds/state/watch-metrics.lock`のflock、保存は同一ディレクトリでのatomic rename。os.Rootでworkspace外のsymlinkを拒否する。

`GET /api/v1/watch/metrics`はこのファイルだけを読み、期間と鮮度フラグを計算する。新規資格・DB・人間のREADME/dailyの更新は不要。

## 配備

Dockerfileは新CLIを同梱し、`watch-metrics-cronjob.yaml`は毎時15分の定義を持つ。既存v0.3.18にはCLIが無いため`suspend: true`。CLIを含む新imageを両archでreleaseした後にimageを揃え、有効化する。project-watchの推論定期実行とは独立し、推論枠を消費しない。

## 変化の見出し

新しいproject-watch生成は36文字以内の`headline`を必須にする。前回成果物・旧schemaの実行中jobも読み込める。旧成果物や、AI生成後に記録が更新された場合は、元記録の最初の強調/文から見出しを表示し、詳細へ遷移する。切り詰めた部分や本文・意味・判断・根拠・生成時刻は`/watch/changes/:project`で読める。指標の集計元は`/watch/activity`で確認する。

## 検証

backend全体race/vet、対象ブラウザ試験、JST月/週境界・ページング・却下除外・失敗時保持・期間外非表示・Zenn total null・外部symlink拒否を検査。Grafanaの実queryを読む回帰試験は`python3 scripts/test-cluster-watch-dashboard.py`（Dockerのprom/prometheus:v3.1.0を使用）。
