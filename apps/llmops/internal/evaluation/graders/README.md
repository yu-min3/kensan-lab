# Versioned grading engine

`Engine{Version, Invoke}.Evaluate(ctx, Case)`は実行状態と評価結果を結び付ける。Invokeはversioned adapter envelopeを受け取り、応答envelopeを返すfunction。workerのallowlist済みadapter実行を注入し、grader自身はshell・runtime・S3を呼ばない。

## 入力と出力

Caseはcase／attempt／request ID、mode、domain schema版、annotation status、expected constraints、output schema、実出力、実行state／reasonを持つ。modeはpreparedまたはrawを明示する。これはrunnerが実行した前処理範囲の記録であり、graderが入力を再取得したり加工したりする設定ではない。

EvaluateはエラーでもEvaluationを返す。呼出側はそのresultを捨てず、caseとattemptに保存する。Statusはpassed／failed／needs_review／not_evaluated。ExecutionState／Reason、annotation status、mode、evaluator版を保持し、Summarizeは全entryをTotalへ数える。

| 状態 | 評価 |
|---|---|
| queued／running／waiting／failed／cancelled | adapterを呼ばずnot_evaluated。実行失敗を採点結果の欠損として隠さない |
| succeeded、schema不適合 | failed。schema=false、schema_mismatch |
| succeeded、schema設定やadapterが壊れている | not_evaluated＋error。モデルの失敗と区別 |
| annotation unreviewed | adapterがpassを返してもneeds_reviewに補正 |
| annotation synthetic | 合成oracleとしてコード合否を出せる。人の確認済みとは記録しない |
| annotation reviewed | domain adapterの確認済み制約を使う。意味を確定できなければneeds_review |

output schemaは32 KiB、出力は1 MiB、adapter envelopeは2 MiBまで。schemaは外部参照を取得せずofflineでcompileする。adapterのrequest ID、protocol、result型を検査し、schema checkの上書きやfalse check付きpassを拒否・補正する。domain checkが空のpassはneeds_reviewにする。

## Domain payload

adapterへ送るpayloadはoutput／expected_constraints／annotation_status。料理型をengineへ入れない。別用途は同じenvelopeに独自のconstraintsとdomain schemaを使う。

Konro v1は以下を扱う。

| 制約 | 意味 |
|---|---|
| expected | 完全な期待recipe。既存importerの型・validationを再利用し、title／servings／ingredients／stepsのexact oracle比較 |
| required_ingredients | 指定した材料行の完全一致による存在検査 |
| required_steps | 指定した工程行の完全一致による存在検査 |
| forbidden_terms | 指定substringがタイトル・分量・材料・工程に含まれないこと |
| preserve_unknown_servings | 分量不明を空文字のまま保持すること |

完全なexpectedがなく部分検査だけが成功した場合はneeds_review。未知のconstraintは黙って無視せずunsupported_constraintsとしてneeds_review。unreviewedの期待値はgoldとして扱わない。旧adapterのconfirmedはreviewed相当の互換入力として残すが、engineのdataset入力はreviewed／synthetic／unreviewedに統一する。

exact oracleは回帰用の厳密な比較であり、言い換えの意味同値を保証する評価ではない。数字のglobal token集合を数量の正解とみなさず、期待recordの材料・工程との対応を比較する。完全期待値に含まれない改変の意味判定は、人の確認や別の注釈設計が必要になる。

## 検証

engineは未完了entryの分母保持、annotation／modeの保持、schema破損、adapter応答不正、未知domain、空check／false checkのpass防止を試験する。Konro adapterは数量の材料間入替、材料追加、工程欠落、不明分量の捏造、未確認oracle、未知制約を試験する。dataset snapshotや実runtimeを通す結合はrunner／integrationの別検証。
