# 比較HTML renderer

`Write(io.Writer, Comparison) error`は、同じ固定dataset・評価器による現行／候補runを一つのHTMLへ描画する。`Decode(io.Reader)`はversion 1のJSON manifestを読む。JSON fixtureの具体例はtestdata/comparison.json。

## 呼出側が用意するもの

artifactの認可・取得はrendererの外で行う。Comparisonにはdatasetのapp／task／release、全table snapshot、2つのrunとprompt／code／evaluator版、caseごとの原文参照・入力、各attemptの出力・状態・評価を渡す。

- Variant.DatasetReleaseIDは共有Dataset.ReleaseIDと一致する必要がある。
- 評価器版は2つのvariantで一致させる。modeはpreparedまたはrawを明示する。
- attemptのcase／run／attempt IDとgrading結果のID・mode・evaluator版・annotation状態・execution状態を照合する。
- caseを片方だけ実行した場合もCasesから消さず、片側Attemptをnilにする。結果未記録として表示する。
- queued／waiting／failed等はnot_evaluatedを使い、実行state／reasonを保持する。

比較対象のcase ID重複、latest snapshot、違うrun／release／評価器、unreviewedのpassを拒否する。Decodeはサイズ16 MiB、UTF-8、未知field・重複key・trailing JSON・過度なnestingも検査する。Writeは描画前に整合を検査するので、入力不正で部分HTMLを成功扱いにしない。

## 表示

全case数と採点対象数、合否・確認待ち・結果未記録を併記する。回帰はpass→fail、改善はfail→passだけに付け、それ以外の未完了・未確認を改善と呼ばない。各caseにRaw object／hash／出典、実際に使った入力、現行／候補の出力、run／attempt、model／usageを表示する。

model／usageのnilは未取得。fixture実行とruntime経由、合成注釈・確認済み注釈・未確認注釈を区別する。合成fixtureの合格を人の実用評価と表示しない。

HTMLはhtml/templateでescapeする。原文・生成文・理由・metadataをtemplate.HTMLへ変換しない。出典URIもtextとして表示し、rendererは外部resourceを取得しない。CSPはscript・外部resource・formを禁止する。CSSはprefers-color-schemeで明暗を切り替え、390pxでは縦並びになる。

HTMLの保存先、CSV／Parquet集計、dataset／run manifestの公開はworker／runnerの責務。rendererからS3を更新しない。

## 検証

Go試験で出力escape、回帰／改善／未記録、全件分母、版・IDの不一致、曖昧JSON、writer失敗を確認する。合成fixtureにはscriptと外部画像の文字列を意図的に含める。

TestFixtureOutputAndEscapeは通常memory上だけで実行する。LLMOPS_REPORT_EXPORTへoperatorが明示したpathがある場合だけ、そのfixture HTMLを書き出す。生成HTMLをローカルHTTPで配信し、tests/browser.cjsへURLを渡すと、Playwrightで390／1440px・明暗のoverflow、active content、外部requestを確認できる。Playwright／browser実行ファイルとスクリーンショット先は環境から指定できる。

このrendererのfixture試験は実runの比較ではない。実dataset snapshot、runtime、保存済みrunとの結合はintegrationで別途確認する。
