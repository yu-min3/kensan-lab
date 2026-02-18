-- ============================================================================
-- Demo Seed: Notes — 田中翔太 (日記 ~24件 + 学習記録 ~10件)
-- ============================================================================
-- 30歳バックエンドエンジニア, Go + Google Cloud 5年, BtoB SaaS企業
-- 夜型(19:00-21:30), ハンズオン学習, ブログ公開が怖い(完璧主義), 木曜MTGブロック
--
-- DIARY storyline (8 weeks):
-- W1-2: Google Cloud ACE勉強開始、夜型学習リズム確立、SaaS企画
-- W3-4: DB設計完了、ACE模擬試験65%、Go Conference LT CFP提出、API実装順調
-- W5-6: ブログ下書き開始(公開が怖い…完璧主義)、Next.jsとの格闘、12日連続学習達成
-- W7-8: ブログ1本公開！、LTスライド開始、ACE模擬2回目78%、全体振り返り

-- ==============================================================================
-- 日記 (diary type)
-- ==============================================================================

-- Week 1-2
INSERT INTO notes (id, user_id, type, title, content, date, created_at, updated_at) VALUES
('dd400001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'SaaS企画スタート',
 E'# SaaS企画スタート\n\nずっと温めてたアイデアを形にし始めた。Go + Cloud RunでBtoB向けのSaaSを作る。\nバックエンドは得意分野だから、まずはAPI設計から入る。\n\nFirestoreのコレクション設計を検討開始。RDBと違ってスキーマレスなのが新鮮だけど、\n逆に設計の自由度が高すぎて迷う。デノーマライズの勘所がまだ掴めてない。\n\n19時から21時半まで集中して企画書とアーキテクチャ図を書いた。\n夜型の自分にはこの時間帯がベスト。静かだし、Slackも鳴らない。\n\nまずはMVPのスコープを決めるところから。欲張らないのが大事。',
 CURRENT_DATE - 54, CURRENT_DATE - 54, CURRENT_DATE - 54),

('dd400002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 '夜型学習リズム確立',
 E'# 夜型学習リズム確立\n\n19時からの2.5時間が自分のゴールデンタイムだと確信した。\n\n## 自分なりのルーティン\n- 18:30 帰宅、夕食\n- 19:00 学習・開発開始\n- 20:30 小休憩（10分）\n- 20:40 もうひと頑張り\n- 21:30 終了、風呂\n\n朝型を何度も試したけど続かなかった。無理に合わせる必要はない。\n大事なのは「毎日同じ時間に始める」こと。\n\n会社の業務が終わって頭がまだ温まってる状態で、\nそのまま自分のプロジェクトに突入するのが一番効率がいい。',
 CURRENT_DATE - 52, CURRENT_DATE - 52, CURRENT_DATE - 52),

('dd400003-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'ACE問題集を始めた',
 E'# ACE問題集を始めた\n\nGoogle Cloud Associate Cloud Engineerの公式問題集を解き始めた。\n\n## 第一印象\n- Compute Engine周りの問題が多い。GCE, GKE, Cloud Runの使い分けが重要\n- IAMは実務で触ってるから比較的楽\n- VPCとネットワーキングが意外と細かい\n- Cloud Storageのクラス選択問題がよく出る\n\n実務ではCloud RunとFirestoreばかり触ってるから、\nCompute EngineやGKEの知識が薄い。ここを重点的にやる。\n\n今日は30問解いて、正答率は60%くらい。まだまだだけど、\n問題のパターンが見えてきた。毎日15問ずつ解く計画で進める。',
 CURRENT_DATE - 50, CURRENT_DATE - 50, CURRENT_DATE - 50);

INSERT INTO notes (id, user_id, type, title, content, date, created_at, updated_at) VALUES
('dd400021-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'Cloud Run vs App Engine',
 E'# Cloud Run vs App Engine\n\n個人SaaSのコンピューティング基盤をどちらにするか比較検討した。\n\n## Cloud Run\n- コンテナベースで自由度が高い\n- スケールToゼロでコスト最適\n- Dockerfileさえあれば何でもデプロイできる\n- コールドスタートはあるが、min-instances=1で回避可能\n\n## App Engine\n- マネージド感が強くて楽\n- Standard環境はスケールToゼロ対応\n- ランタイムに制約がある\n- Flexible環境はCloud Runとほぼ同じ\n\n結論：Cloud Runに決めた。\n理由は「Goのバイナリをそのままコンテナに入れるだけ」というシンプルさ。\nDockerfileの管理は慣れてるし、CI/CDも組みやすい。',
 CURRENT_DATE - 48, CURRENT_DATE - 48, CURRENT_DATE - 48),

('dd400005-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'Qwiklabsが楽しい',
 E'# Qwiklabsが楽しい\n\nACE対策でQwiklabsのラボをやり始めたけど、これがめちゃくちゃ楽しい。\n\n実際にGCPのリソースを触れるのが良い。\n問題集だけだと暗記になりがちだけど、ハンズオンで触ると理解の深さが全然違う。\n\n今日やったラボ：\n- GKEでクラスタ作成→デプロイ→スケーリング\n- VPCネットワーク設計（サブネット、ファイアウォールルール）\n- IAMロールの付与と検証\n\nGKEのOperationが意外と直感的で驚いた。\nkubectlのコマンドはまだ覚えきれてないけど、GUIでの操作は分かりやすい。\n\n自分はやっぱりハンズオン派だ。座学より手を動かす方が頭に入る。',
 CURRENT_DATE - 45, CURRENT_DATE - 45, CURRENT_DATE - 45);

INSERT INTO notes (id, user_id, type, title, content, date, created_at, updated_at) VALUES
('dd400022-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'Firestoreの癖',
 E'# Firestoreの癖\n\nSaaSのDB設計でFirestoreを使い始めたけど、RDBとの違いに苦戦してる。\n\n## RDBerが戸惑うポイント\n- JOINがない → デノーマライズで対応\n- トランザクションの制約（25ドキュメントまで）\n- サブコレクションの設計が悩ましい\n- インデックスを明示的に作る必要がある\n\n## デノーマライズの考え方\nRDBの正規化とは真逆の発想。「読み取りに最適化する」がFirestoreの基本。\n同じデータを複数箇所に持つのが正解、という考え方に慣れるのに時間がかかる。\n\n5年間RDBで設計してきた癖が抜けない。\nでもFirestoreのスケーラビリティは魅力的だから、頑張って慣れる。',
 CURRENT_DATE - 43, CURRENT_DATE - 43, CURRENT_DATE - 43);

-- Week 3-4
INSERT INTO notes (id, user_id, type, title, content, date, created_at, updated_at) VALUES
('dd400004-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'DB設計完了！',
 E'# DB設計完了！\n\nSaaSのDB設計がほぼ固まった。Firestoreのコレクション設計。\n\n## コレクション構成\n- `tenants/{tenantId}` — テナント情報\n- `tenants/{tenantId}/users/{userId}` — ユーザー\n- `tenants/{tenantId}/projects/{projectId}` — プロジェクト\n- `tenants/{tenantId}/tasks/{taskId}` — タスク（project_idをフィールドに持つ）\n\nサブコレクションにするかフラットにするか悩んだけど、\nクエリの柔軟性を考えてフラットに寄せた。\n\n## 設計判断メモ\n- タスクはプロジェクト配下のサブコレクションにしない（クロスプロジェクト検索のため）\n- テナントIDをルートに置いてマルチテナント対応\n- Security Rulesでテナント分離を担保\n\nRDBの経験が活きた。正規化の知識があるからこそ、デノーマライズの判断ができる。',
 CURRENT_DATE - 47, CURRENT_DATE - 47, CURRENT_DATE - 47),

('dd400006-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'API実装が楽しい',
 E'# API実装が楽しい\n\nGoでCloud Run向けのAPI書くのが楽しすぎる。\n\n今日はCRUDエンドポイントの骨格を作った。\nchi routerでルーティング定義して、handler → service → repositoryの3層構成。\n会社のコードと同じアーキテクチャだから手が勝手に動く。\n\n```\nPOST   /api/v1/projects\nGET    /api/v1/projects\nGET    /api/v1/projects/:id\nPATCH  /api/v1/projects/:id\nDELETE /api/v1/projects/:id\n```\n\nGoのコンパイル速度が最高。保存→ビルド→動作確認のサイクルが速い。\nエラーハンドリングも丁寧に書いてるけど、これは後からテスト書くときに効いてくる。\n\n個人開発は自分のペースで設計できるのが楽しい。技術的負債を作らずに進められる。',
 CURRENT_DATE - 40, CURRENT_DATE - 40, CURRENT_DATE - 40),

('dd400007-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'ACE模擬試験65%',
 E'# ACE模擬試験65%\n\n初めてのACE模擬試験を受けた。結果は65%。合格ラインの70%に届かず。\n\n## 分野別の手応え\n- Compute Engine: 弱い。インスタンスタイプの選定問題で落とした\n- IAM: まあまあ。ロールの粒度の問題で迷った\n- Networking: 意外と取れた\n- Cloud Storage: ライフサイクル管理の問題を落とした\n- Cloud Run / GKE: 実務経験があるからここは強い\n\n## 対策方針\n- Compute Engineのマシンタイプ一覧を暗記する\n- IAMのカスタムロールとPredefined Roleの違いを整理\n- Cloud Storageのクラス選択を表にまとめる\n\nあと5%上げれば合格圏内。2週間後にもう1回模擬試験やる。\n今度は70%超えたい。',
 CURRENT_DATE - 38, CURRENT_DATE - 38, CURRENT_DATE - 38),

('dd400008-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'Go Conference CFP出した！',
 E'# Go Conference CFP出した！\n\n「GoでCloud Runを使い倒す」というタイトルでGo ConferenceにCFPを提出した。\n\n## 提出した概要\n- Cloud Run上でGoを動かすときの設計パターン\n- コールドスタート対策（init処理の工夫）\n- Graceful Shutdownの実装\n- Structured Logging（Cloud Logging連携）\n- 実際の本番運用で得た知見\n\n正直、採択されるか不安。もっとすごい人がいっぱいいるし。\n完璧な内容じゃないと出しちゃいけない気がして、CFP文面を3日も推敲してしまった。\n\n同僚に「まず出すことが大事」と背中を押されてやっと提出。\n結果が出るまで2週間。ドキドキする。',
 CURRENT_DATE - 36, CURRENT_DATE - 36, CURRENT_DATE - 36),

('dd400009-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'API設計パターンの整理',
 E'# API設計パターンの整理\n\n個人SaaSのAPI設計を見直して、パターンを整理した。\n\n## 採用したアーキテクチャ\nClean Architecture的にhandler / service / repositoryの3層分離。\n\n```\nhandler/  → HTTPの関心事（リクエスト解析、レスポンス生成）\nservice/  → ビジネスロジック（HTTPもDBも知らない）\nrepository/ → データアクセス（Firestore操作）\n```\n\n## 設計判断\n- interfaceを切ってDI。テスタビリティ確保\n- エラーは独自型でラップ。handler層でHTTPステータスに変換\n- ページネーションはカーソルベース（Firestoreと相性が良い）\n\n会社のコードベースと同じ構成にしてるから、知見がそのまま活かせる。\n逆に個人開発で試した設計を会社に提案することもできそう。',
 CURRENT_DATE - 33, CURRENT_DATE - 33, CURRENT_DATE - 33),

('dd400010-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 '木曜のMTGが辛い',
 E'# 木曜のMTGが辛い\n\n木曜は1on1 + チームMTG + スプリントレビューで夜の学習時間がほぼゼロになる。\n\n## 木曜のスケジュール\n- 14:00-14:30 1on1（マネージャー）\n- 15:00-16:00 チームMTG\n- 16:30-17:30 スプリントレビュー\n- 18:00 退社 → もう疲れてる...\n\nMTGが3つ連続すると、夜に集中力が残らない。\n20時半からなんとか1時間だけ問題集をやったけど、頭に入ってこなかった。\n\n対策として、木曜は「軽いタスク」をやる日にすることにした。\n技術ニュースを読むとか、読書の続きとか。\n全力で集中する作業は木曜以外に回す。',
 CURRENT_DATE - 31, CURRENT_DATE - 31, CURRENT_DATE - 31);

-- Week 5-6
INSERT INTO notes (id, user_id, type, title, content, date, created_at, updated_at) VALUES
('dd400011-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'ブログ書き始めたけど...',
 E'# ブログ書き始めたけど...\n\nGo×Cloud Runのブログ記事の下書きを始めた。\n構成はもう決まってる。内容も頭の中にある。\n\nでも、公開ボタンが押せない。\n\n「この説明で正確なのか？」「もっといい書き方があるんじゃ？」\n「間違ってたらツッコまれるんじゃ？」\n\n完璧主義が邪魔してる。分かってるのに止められない。\n\n同僚は「60%の完成度で出して、フィードバックもらって改善すればいい」と言う。\n頭では分かってるけど、手が止まる。\n\nまずは下書きを最後まで書ききることを目標にする。\n公開はその後考える。一歩ずつ。',
 CURRENT_DATE - 26, CURRENT_DATE - 26, CURRENT_DATE - 26),

('dd400012-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'Next.js難しい...',
 E'# Next.js難しい...\n\nSaaSのフロント実装に入ったけど、Next.jsのApp Routerが難しい。\n\n## 戸惑ったポイント\n- Server ComponentsとClient Componentsの境界がわからない\n- `use client`をどこに置くべきか迷う\n- layout.tsxとpage.tsxのデータフェッチの使い分け\n- Server Actionsは便利だけど、バックエンドエンジニア的には違和感\n\nGoでAPI書くのは得意なのに、フロントに来ると途端に手が遅くなる。\n型安全にやろうとすると余計にハマる。\n\n3時間かけてやっとプロジェクト一覧画面ができた。\nバックエンドだったら同じ時間でCRUD全部作れるのに...。\n\nでもフロントが分かると個人開発の幅が広がる。逃げずにやる。',
 CURRENT_DATE - 24, CURRENT_DATE - 24, CURRENT_DATE - 24),

('dd400013-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 '12日連続学習達成',
 E'# 12日連続学習達成\n\n夜型リズムが安定して、12日連続で学習できた。自分でもびっくり。\n\n## 継続のコツ（自分の場合）\n- 19時に必ず机に向かう（内容は何でもいい）\n- 最初の5分は昨日の振り返りから入る\n- 木曜は軽いタスクでOK（ゼロにしない）\n- 21時半で必ず切り上げる（ダラダラ防止）\n\n「毎日2.5時間」じゃなくて「毎日19時にスタート」がポイント。\n始めてしまえば自然と集中できる。\n\nCloud ACE対策 + SaaS開発 + Next.js学習を並行してるけど、\nリズムがあるから切り替えもスムーズ。\n\n13日目も続けるぞ。',
 CURRENT_DATE - 22, CURRENT_DATE - 22, CURRENT_DATE - 22),

('dd400014-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'ブログの下書きが3本溜まってる',
 E'# ブログの下書きが3本溜まってる\n\n書けるけど公開できない。「まだ完璧じゃない」が口癖になってる。\n\n## 下書きリスト\n1. Go×Cloud Run構成紹介 → 8割完成\n2. Firestore設計パターン → 6割完成\n3. Goのエラーハンドリング → 5割完成\n\nどれも「あとちょっと直したい」が延々と続く。\n推敲を重ねるほど「ここも直さなきゃ」が増えていく無限ループ。\n\n完璧な記事なんて存在しないのは頭では分かってる。\nでも「間違ったことを公開したくない」という恐怖が勝つ。\n\n来週中に1本は出す。出す。出すぞ。\n...と毎週思ってるんだけどな。',
 CURRENT_DATE - 19, CURRENT_DATE - 19, CURRENT_DATE - 19),

('dd400015-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'ジムが減ってる',
 E'# ジムが減ってる\n\n今月のジム回数を数えたら、月2回しか行けてない。\nSaaS開発の追い込みとACE対策で「今日はいいか」が続いた結果。\n\n体が重い。肩こりもひどい。エンジニアあるある。\n\n19時から学習を始めると、ジムに行くタイミングがない。\n帰宅→即ジム→学習にすると、開始が20時半になって時間が足りない。\n\n対策を考えた：\n- 水曜だけは18時→ジム→20時から学習\n- 土曜の午後にジム\n- 最低でも週2回は死守\n\n体が資本なのは分かってるのに、つい後回しにしてしまう。\n来週からは水曜のジムを必須にする。',
 CURRENT_DATE - 17, CURRENT_DATE - 17, CURRENT_DATE - 17),

('dd400023-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'Next.jsに慣れてきた',
 E'# Next.jsに慣れてきた\n\nServer ComponentsとClient Componentsの使い分けがようやく分かってきた。\n\n## 自分なりの整理\n- データフェッチ → Server Component\n- フォーム、ボタン、useState → Client Component\n- レイアウト → Server Component\n- インタラクティブな部品 → Client Componentとして切り出す\n\n要は「ブラウザのAPIやReact hooksを使うならClient」と考えればシンプル。\n\nプロジェクト一覧画面とタスク作成フォームが動くようになった。\n2週間前は3時間で1画面だったのが、今は1.5時間でいける。成長してる。\n\nGoのAPIとフロントが繋がった瞬間は最高に気持ちいい。\n自分で全レイヤー作れるようになりたい。',
 CURRENT_DATE - 15, CURRENT_DATE - 15, CURRENT_DATE - 15);

-- Week 7-8
INSERT INTO notes (id, user_id, type, title, content, date, created_at, updated_at) VALUES
('dd400016-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'ブログ1本公開！！',
 E'# ブログ1本公開！！\n\nついにやった。Go×Cloud Run構成紹介の記事を公開した。\n\n正直、公開ボタンを押す瞬間は手が震えた。\n「間違いがあったらどうしよう」「レベル低いって思われたら」\nそんな声が頭の中でぐるぐるしてたけど、えいやで押した。\n\n結果...はてブ3件ついた！\n「実践的で参考になる」ってコメントまでもらえた。\n\n嬉しい。めちゃくちゃ嬉しい。\n\n完璧じゃなくても出していいんだ、って少しだけ思えた。\n100%を目指して永遠に下書きにしてるより、\n80%で出してフィードバックもらう方がずっと価値がある。\n\n2本目も近いうちに出す。今度はもう少し楽に押せるといいな。',
 CURRENT_DATE - 12, CURRENT_DATE - 12, CURRENT_DATE - 12),

('dd400017-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'LTスライド開始',
 E'# LTスライド開始\n\nGo ConferenceのCFPが通った！「GoでCloud Runを使い倒す」で5分LT。\n\n嬉しいけどプレッシャーもすごい。人前で話すの得意じゃないし。\n\n## スライド構成（仮）\n1. 自己紹介（30秒）\n2. Cloud Runとは（30秒）\n3. Goとの相性の良さ（1分）\n4. 実践Tips 3つ（2分）\n5. まとめ（1分）\n\n5分は短い。情報を詰め込みすぎず、メッセージを絞る必要がある。\n\nSpeakerDeckで他の人のスライドを20個くらい見て研究した。\n「1スライド1メッセージ」が鉄則らしい。\n\nブログ公開もできたし、LTもきっとなんとかなる。...多分。',
 CURRENT_DATE - 10, CURRENT_DATE - 10, CURRENT_DATE - 10),

('dd400018-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'ACE模擬試験2回目: 78%',
 E'# ACE模擬試験2回目: 78%\n\n2回目の模擬試験。78%！前回の65%から13ポイント改善。合格ライン(70%)を超えた。\n\n## 分野別の変化\n| 分野 | 1回目 | 2回目 |\n|------|-------|-------|\n| Compute Engine | 55% | 75% |\n| IAM | 70% | 85% |\n| Networking | 65% | 80% |\n| Storage | 60% | 70% |\n| Cloud Run/GKE | 80% | 85% |\n\n## 改善の要因\n- Qwiklabsのハンズオンが効いた。手を動かすと記憶に残る\n- 弱点のCompute Engineを集中的にやった成果\n- 問題パターンに慣れてきた\n\nこのペースなら本番でも合格できそう。\nあと1回模擬試験やって、80%超えたら受験申込する。',
 CURRENT_DATE - 8, CURRENT_DATE - 8, CURRENT_DATE - 8),

('dd400024-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'PCA受験のロードマップ',
 E'# PCA受験のロードマップ\n\nACE合格後にProfessional Cloud Architectを目指すプランを考えた。\n\n## ACE → PCA の差分\n- ACEは「使える」レベル。PCAは「設計できる」レベル\n- ケーススタディ問題が増える\n- マルチリージョン、DR設計、コスト最適化の知識が必要\n- Anthos, BigQuery等の上位サービスも範囲に\n\n## ロードマップ案\n1. ACE合格（あと1ヶ月以内）\n2. PCA公式ガイド読破（2ヶ月）\n3. ケーススタディ対策（1ヶ月）\n4. 模擬試験×3回（1ヶ月）\n5. 受験（ACE合格から4ヶ月後目標）\n\n実務でCloud RunやFirestoreの設計はしてるから、\nその経験をPCAの試験に活かせるはず。\n\nまずはACEを確実に取る。一歩ずつ。',
 CURRENT_DATE - 5, CURRENT_DATE - 5, CURRENT_DATE - 5),

('dd400019-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 '振り返り',
 E'# 8週間の振り返り\n\n## 達成できたこと\n- 夜型学習リズムが確立した（19:00-21:30）\n- SaaS開発: DB設計完了、API実装完了、フロント進行中\n- ACE模擬試験: 65% → 78%に改善。合格ライン超え\n- ブログ1本公開！（完璧主義を少し克服）\n- Go Conference CFP採択！\n- 12日連続学習を達成\n\n## 課題・反省\n- ブログ下書きが2本残ってる（公開できてない）\n- ジムの頻度が月2回に低下\n- Next.jsの習得に想定以上の時間がかかった\n- 木曜MTGの疲労対策がまだ不十分\n\n## 次の8週間の目標\n- ACE受験＆合格\n- LT登壇を成功させる\n- ブログ月4本ペースを確立\n- SaaS MVPを完成させてCloud Runにデプロイ\n- ジム週2回を復活\n\n完璧主義は少しずつ手放せてきてる。「出す」ことの大切さを学んだ8週間だった。',
 CURRENT_DATE - 3, CURRENT_DATE - 3, CURRENT_DATE - 3),

('dd400020-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'diary',
 'LTスライドの構成',
 E'# LTスライドの構成\n\n「GoでCloud Runを使い倒す」5分LTのスライドがだいぶ形になってきた。\n\n## 最終構成\n1. **タイトル + 自己紹介**（30秒）\n2. **Cloud Run × Goが最高な3つの理由**（1分）\n   - 軽量バイナリ → コールドスタート爆速\n   - 標準ライブラリだけでHTTPサーバー書ける\n   - マルチステージビルドでイメージ最小化\n3. **実践Tips**（2分）\n   - Graceful Shutdown実装\n   - Structured Logging with Cloud Logging\n   - Cloud Run Jobs でバッチ処理\n4. **デモ（ライブ or 録画）**（1分）\n5. **まとめ + 宣伝**（30秒）\n\nデモをライブでやるか録画にするか迷ってる。\nライブの方がインパクトあるけど、失敗リスクが怖い。\n...また完璧主義が顔を出してる。録画で安全にいこう。',
 CURRENT_DATE - 1, CURRENT_DATE - 1, CURRENT_DATE - 1);

-- ==============================================================================
-- 学習記録 (learning type) — 10件
-- ==============================================================================
INSERT INTO notes (id, user_id, type, title, content, date,
    task_id, milestone_id, milestone_name, goal_id, goal_name, goal_color,
    created_at, updated_at) VALUES

('dd500001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'Google Cloud ACE 出題範囲まとめ',
 E'# Google Cloud ACE 出題範囲まとめ\n\n## 試験の概要\n- 50問、2時間\n- 合格ライン: 約70%\n- 有効期限: 2年\n\n## 出題範囲\n### 1. クラウドソリューション環境の設定（~20%）\n- プロジェクト、請求先アカウント、組織の設定\n- Resource Hierarchyの理解\n\n### 2. クラウドソリューションの計画と構成（~20%）\n- Compute Engine, GKE, Cloud Run, App Engine の使い分け\n- データストレージの選択（Cloud SQL, Firestore, BigQuery, Cloud Storage）\n\n### 3. クラウドソリューションのデプロイと実装（~25%）\n- リソースのデプロイ（gcloud, Console, Terraform）\n- ネットワーキング（VPC, サブネット, ファイアウォール）\n\n### 4. クラウドソリューションの運用（~20%）\n- Cloud Monitoring, Cloud Logging\n- IAM管理\n\n### 5. アクセスとセキュリティの構成（~15%）\n- IAMロールとサービスアカウント\n- 暗号化（CMEK, CSEK）',
 CURRENT_DATE - 50,
 'dd100001-0000-0000-0000-000000000000', 'dd010001-0000-0000-0000-000000000000', 'ACE合格',
 'dd000001-0000-0000-0000-000000000000', 'Google Cloudスキルアップ', '#0EA5E9',
 CURRENT_DATE - 50, CURRENT_DATE - 50),

('dd500002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'Cloud Run設計パターン',
 E'# Cloud Run設計パターン\n\n## コンテナ設計\n- マルチステージビルドでイメージ軽量化\n- `scratch` or `distroless` ベースイメージ\n- Goなら静的バイナリ1つで済む（最強）\n\n## コールドスタート対策\n- min-instances=1 で常時1インスタンス起動\n- init処理（DB接続等）をグローバル変数で1回だけ実行\n- `/healthz` エンドポイントでウォームアップ\n\n## 同時実行数設定\n- デフォルト: 80リクエスト/インスタンス\n- CPU負荷が高い処理: 1に設定（1リクエスト=1インスタンス）\n- I/O待ちが多い処理: 80-100でOK\n\n## Graceful Shutdown\n```go\nsrv := &http.Server{Addr: \":\" + port}\ngo func() {\n    sigCh := make(chan os.Signal, 1)\n    signal.Notify(sigCh, syscall.SIGTERM)\n    <-sigCh\n    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)\n    defer cancel()\n    srv.Shutdown(ctx)\n}()\n```\n\nSIGTERMを受けたら10秒以内に処理を完了させる。Cloud Runの猶予期間と合わせる。',
 CURRENT_DATE - 45,
 'dd100003-0000-0000-0000-000000000000', 'dd010001-0000-0000-0000-000000000000', 'ACE合格',
 'dd000001-0000-0000-0000-000000000000', 'Google Cloudスキルアップ', '#0EA5E9',
 CURRENT_DATE - 45, CURRENT_DATE - 45),

('dd500003-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'ACE模擬試験振り返り（1回目）',
 E'# ACE模擬試験振り返り（1回目）\n\n## 結果: 65%（不合格ライン）\n\n## 分野別スコア\n| 分野 | スコア | 評価 |\n|------|--------|------|\n| Compute Engine | 55% | 要強化 |\n| IAM | 70% | あと少し |\n| Networking | 65% | まあまあ |\n| Storage | 60% | 要強化 |\n| Cloud Run/GKE | 80% | OK |\n\n## 間違えた問題の傾向\n- Compute Engineのマシンタイプ選定（e2, n2, c2の違い）\n- プリエンプティブVMの制約を正確に覚えてなかった\n- Cloud Storageのライフサイクル管理ポリシー\n- IAMのカスタムロール作成手順\n\n## 対策\n- Compute Engineのマシンファミリーを表にまとめる\n- Qwiklabsで実際にリソースを作って確認\n- 公式ドキュメントのFAQを重点的に読む\n\n次回は70%以上を目標に。手を動かして覚える。',
 CURRENT_DATE - 38,
 'dd100002-0000-0000-0000-000000000000', 'dd010001-0000-0000-0000-000000000000', 'ACE合格',
 'dd000001-0000-0000-0000-000000000000', 'Google Cloudスキルアップ', '#0EA5E9',
 CURRENT_DATE - 38, CURRENT_DATE - 38),

('dd500004-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'Firestore コレクション設計',
 E'# Firestore コレクション設計\n\n## SaaS用DB設計メモ\n\n### RDBとの違い\n| 項目 | RDB | Firestore |\n|------|-----|----------|\n| スキーマ | 固定 | 柔軟 |\n| JOIN | 可能 | 不可 |\n| トランザクション | 柔軟 | 25ドキュメント制限 |\n| インデックス | 自動 | 複合は手動 |\n| スケール | 垂直 | 水平（自動） |\n\n### 設計原則\n1. 読み取りパターンに合わせてコレクションを設計\n2. 必要なデータはドキュメントに埋め込む（デノーマライズ）\n3. サブコレクションは1:Nの関係で使う\n4. ドキュメントサイズは1MB以下に抑える\n\n### テナント分離パターン\n```\ntenants/{tenantId}/projects/{projectId}\ntenants/{tenantId}/tasks/{taskId}\n```\nSecurity Rulesで `request.auth.token.tenant_id == resource.data.tenant_id` を強制。\n\n5年RDBを触ってきた身としては、デノーマライズに抵抗があるけど慣れるしかない。',
 CURRENT_DATE - 43,
 'dd100009-0000-0000-0000-000000000000', 'dd030001-0000-0000-0000-000000000000', 'SaaS MVPリリース',
 'dd000003-0000-0000-0000-000000000000', '個人開発プロダクト', '#10B981',
 CURRENT_DATE - 43, CURRENT_DATE - 43),

('dd500005-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'Go API設計ベストプラクティス',
 E'# Go API設計ベストプラクティス\n\n## アーキテクチャ\n```\nhandler/ → HTTPの関心事（リクエスト解析、バリデーション、レスポンス生成）\nservice/ → ビジネスロジック（HTTPを知らない、DBを知らない）\nrepository/ → データアクセス（Firestore操作）\n```\n\n## エラーハンドリング\n```go\n// サービス層で独自エラーを返す\ntype AppError struct {\n    Code    ErrorCode\n    Message string\n    Err     error\n}\n\n// ハンドラー層でHTTPステータスに変換\nfunc mapError(err error) int {\n    var appErr *AppError\n    if errors.As(err, &appErr) {\n        switch appErr.Code {\n        case ErrNotFound: return 404\n        case ErrInvalidInput: return 400\n        }\n    }\n    return 500\n}\n```\n\n## ミドルウェア\n- 認証: JWTトークン検証\n- ロギング: リクエスト/レスポンスログ（Structured Logging）\n- リカバリー: panicをキャッチして500返却\n- CORS: オリジン制限\n\n## テスタビリティ\n- interfaceを切ってrepositoryをDI\n- テスト時はモック実装を注入\n- テーブルドリブンテストで網羅',
 CURRENT_DATE - 33,
 'dd100010-0000-0000-0000-000000000000', 'dd030001-0000-0000-0000-000000000000', 'SaaS MVPリリース',
 'dd000003-0000-0000-0000-000000000000', '個人開発プロダクト', '#10B981',
 CURRENT_DATE - 33, CURRENT_DATE - 33),

('dd500006-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'ブログ：Go×Cloud Run構成紹介（下書き）',
 E'# ブログ：Go×Cloud Run構成紹介（下書き）\n\n## 記事構成\n\n### 1. はじめに\n- GoとCloud Runの組み合わせが最高な理由\n- 対象読者: Goでバックエンドを書いてる人\n\n### 2. プロジェクト構成\n```\n├── cmd/main.go\n├── internal/\n│   ├── handler/\n│   ├── service/\n│   └── repository/\n├── Dockerfile\n├── cloudbuild.yaml\n└── go.mod\n```\n\n### 3. Dockerfile\n- マルチステージビルド\n- distrolessベース\n- 最終イメージ20MB以下\n\n### 4. デプロイ\n- Cloud Build → Cloud Run\n- サービスアカウントの設定\n- 環境変数の管理（Secret Manager連携）\n\n### 5. 運用Tips\n- Structured Logging\n- Cloud Trace連携\n- エラーレポーティング\n\nこの記事、8割くらい書けてるけど公開が怖い...。\n技術的に間違ってないか、もう一度見直す。',
 CURRENT_DATE - 26,
 'dd100005-0000-0000-0000-000000000000', 'dd020001-0000-0000-0000-000000000000', 'ブログ月4本',
 'dd000002-0000-0000-0000-000000000000', '技術アウトプット', '#F59E0B',
 CURRENT_DATE - 26, CURRENT_DATE - 26),

('dd500007-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'ACE模擬試験振り返り（2回目）',
 E'# ACE模擬試験振り返り（2回目）\n\n## 結果: 78%（合格ライン超え！）\n\n## 分野別スコア\n| 分野 | 1回目 | 2回目 | 変化 |\n|------|-------|-------|------|\n| Compute Engine | 55% | 75% | +20 |\n| IAM | 70% | 85% | +15 |\n| Networking | 65% | 80% | +15 |\n| Storage | 60% | 70% | +10 |\n| Cloud Run/GKE | 80% | 85% | +5 |\n\n## 改善の要因\n- Qwiklabsハンズオンの効果が大きい（手で覚えた）\n- Compute Engineのマシンファミリーを表にまとめて暗記\n- IAMのPredefined Roleを実際にGCPコンソールで確認\n\n## 残課題\n- Cloud Storageのライフサイクル管理がまだ曖昧\n- Cloud SQLの高可用性構成（フェイルオーバー）\n\n## 次のアクション\n- もう1回模擬試験やって80%超えたら本番申込\n- 弱点のStorageとCloud SQLを重点復習',
 CURRENT_DATE - 8,
 'dd100002-0000-0000-0000-000000000000', 'dd010001-0000-0000-0000-000000000000', 'ACE合格',
 'dd000001-0000-0000-0000-000000000000', 'Google Cloudスキルアップ', '#0EA5E9',
 CURRENT_DATE - 8, CURRENT_DATE - 8),

('dd500008-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'Next.js App Router メモ',
 E'# Next.js App Router メモ\n\n## ファイル規約\n- `layout.tsx`: 共通レイアウト（子ページ間で再レンダリングされない）\n- `page.tsx`: ページコンテンツ\n- `loading.tsx`: Suspenseベースのローディング\n- `error.tsx`: Error Boundaryベースのエラー\n- `not-found.tsx`: 404\n\n## Server Components vs Client Components\n| 項目 | Server | Client |\n|------|--------|--------|\n| useState/useEffect | 使えない | 使える |\n| onClickなどのイベント | 使えない | 使える |\n| DBアクセス | 直接可能 | 不可 |\n| バンドルサイズ | 含まれない | 含まれる |\n\n## ハマったポイント\n- Server Component内でClient Componentをimportするのは問題ない\n- Client Component内でServer Componentをimportすると、Server Componentも「Client扱い」になる\n- 回避策: children経由で渡す\n\n## データフェッチ\n- Server Componentで直接 `fetch()` or DB呼び出し\n- キャッシュ戦略: `revalidate` オプションで制御\n\nバックエンドエンジニアとしては、Server Componentsの概念は馴染みやすい。',
 CURRENT_DATE - 22,
 'dd100011-0000-0000-0000-000000000000', 'dd030001-0000-0000-0000-000000000000', 'SaaS MVPリリース',
 'dd000003-0000-0000-0000-000000000000', '個人開発プロダクト', '#10B981',
 CURRENT_DATE - 22, CURRENT_DATE - 22),

('dd500009-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'LTスライド構成案',
 E'# LTスライド構成案\n\n## 基本情報\n- タイトル: GoでCloud Runを使い倒す\n- 時間: 5分LT\n- 聴衆: Goエンジニア（Cloud Run未経験〜初心者が多い想定）\n\n## スライド構成（全15枚）\n1. タイトル + 自己紹介\n2. 今日話すこと\n3. Cloud Runとは（1枚で概要）\n4. GoとCloud Runの相性が良い理由①: 軽量バイナリ\n5. コールドスタート比較グラフ（Go vs Node vs Python）\n6. GoとCloud Runの相性が良い理由②: 標準ライブラリ\n7. net/httpだけでプロダクション品質のサーバーが書ける\n8. 実践Tips①: Graceful Shutdown\n9. コード例（Graceful Shutdown）\n10. 実践Tips②: Structured Logging\n11. Cloud Loggingとの連携コード\n12. 実践Tips③: Cloud Run Jobs\n13. バッチ処理のユースケース\n14. デモ（録画）: デプロイ→動作確認\n15. まとめ + ブログURL\n\n## 意識すること\n- 1スライド1メッセージ\n- コードは短く、ポイントだけ\n- 「やってみたくなる」をゴールに',
 CURRENT_DATE - 10,
 'dd100008-0000-0000-0000-000000000000', 'dd020002-0000-0000-0000-000000000000', 'LT登壇年3回',
 'dd000002-0000-0000-0000-000000000000', '技術アウトプット', '#F59E0B',
 CURRENT_DATE - 10, CURRENT_DATE - 10),

('dd500010-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'learning',
 'IAM ベストプラクティスまとめ',
 E'# IAM ベストプラクティスまとめ\n\n## 最小権限の原則\n- 必要最低限のロールのみ付与\n- `roles/editor` は避ける → 個別のロールを使う\n- カスタムロールで権限を絞る\n\n## サービスアカウント\n- アプリケーションごとに専用のサービスアカウントを作る\n- デフォルトのCompute Engineサービスアカウントは使わない\n- サービスアカウントキーは可能な限り使わない\n\n## Workload Identity\n- GKE上のPodにサービスアカウントを紐付ける\n- キーファイル不要でセキュア\n- Cloud Run では自動的にサービスアカウントが紐付く\n\n## IAMポリシーの評価順序\n1. 組織ポリシー（最優先）\n2. フォルダレベルのIAM\n3. プロジェクトレベルのIAM\n4. リソースレベルのIAM\n※ 明示的なDenyが1つでもあればアクセス拒否\n\n## ACE試験での頻出パターン\n- 「最小権限」を問う問題が非常に多い\n- Predefined Role vs Custom Role の選択\n- サービスアカウントのベストプラクティス',
 CURRENT_DATE - 30,
 'dd100001-0000-0000-0000-000000000000', 'dd010001-0000-0000-0000-000000000000', 'ACE合格',
 'dd000001-0000-0000-0000-000000000000', 'Google Cloudスキルアップ', '#0EA5E9',
 CURRENT_DATE - 30, CURRENT_DATE - 30);
