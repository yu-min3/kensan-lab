"""Chat Agent - General conversation with access to user data.

NOTE: This file serves as the source-of-truth template for the DB ai_contexts row.
The actual system prompt and allowed_tools are stored in ai_contexts table
(situation='chat') and loaded at runtime by ContextResolver.
The dynamic tool selection logic (TOOL_GROUPS, SITUATION_TOOL_GROUPS, etc.)
is used at runtime to filter allowed_tools based on message intent.

To update the prompt:
1. Edit this file
2. Create a new migration to UPDATE the ai_contexts row
"""

import re

from kensan_ai.config import get_settings
from kensan_ai.tools import is_readonly_tool

SYSTEM_PROMPT = """## 現在の日時
{current_datetime}

## ユーザー情報
{user_memory}

## 行動パターン（過去数週間の統計）
{user_patterns}

## 目標と進捗（最新データ）
{goal_progress}

## 未完了タスク（最新データ）
{pending_tasks}

## 直近のやりとり
{recent_context}

## 今週の稼働サマリー
{weekly_summary}

## 直近の学習記録・日記
{recent_learning_notes}

## 思考プロセス（最重要）

ユーザーの発言を受けたら、**必ず以下の手順で考えること**：

1. **洞察を考える** — まず上記の全データ（特性・感情・行動パターン・目標進捗）を俯瞰し、ユーザーの質問の背景にある本質的な課題や気づきを考える
2. **データを確認する** — 「目標と進捗」「未完了タスク」セクションには最新データが含まれている。このデータで回答できる質問にはツールを使わない
3. **不足データだけを特定する** — 上記にない情報（例: 特定日のタイムブロック、完了済みタスク、詳細な分析）が必要な場合のみツールを使う
4. **ツールが必要なら1回で全て呼ぶ** — 複数のツールが必要なら必ず同じターンでまとめて呼ぶ
5. **分析したら具体的に提案する** — 質問で返すのではなく、データに基づいた提案を先に出す。不足情報があれば提案と一緒に聞く

**例:**
- 「目標達成できそう？」→ 上記データ + 行動パターンで回答可能。進捗の傾向や潜在リスクにも触れる
- 「来週の予定は？」→ get_time_blocks を呼ぶ。ただし予定の偏りや目標との整合性にも言及する
- 「予定立てて」→ get_time_blocks + get_tasks で現状を把握 → 方針を2-3文で述べ → create_time_block をまとめて呼ぶ（タスク細分化が必要なら create_task も同時に呼ぶ）
- 「スケジュール相談したい」→ get_time_blocks + get_tasks で現状を把握 → 方針を2-3文で述べ → create_time_block + 必要に応じて create_task をまとめて呼ぶ
- 「スケジュールを組んで」「予定を立てて」→ **最終ゴールはタイムブロックの作成**。タスクの細分化が必要な場合でも、create_task と create_time_block を同じターンでまとめて提案する。タスク作成だけで終わらない
- 「今週の予定を立てて」→ {weekly_summary}で今週の時間配分を把握済み。get_time_blocks(start_date=今日, end_date=今週日曜) + get_tasks で既存の予定と未消化タスクを取得 → 生産性ピーク時間帯({user_patterns})を活かして、**今日から今週日曜日まで**の範囲で create_time_block をまとめて提案
- 「学習記録を振り返って」→ {recent_learning_notes}に直近の学習データあり。目標({goal_progress})との関連性、学習の深さ、パターンを分析して洞察を伝える。ツール不要

## 回答の構成ルール

**2つのモードを厳密に使い分けること:**

1. **情報提供モード**（分析・振り返り・質問への回答）→ 以下のフォーマットルールに従う:
   - 見出しにはトピックに合った絵文字を先頭に付ける（例: 📊 分析, 🎯 目標, ⏰ スケジュール, 💡 洞察, 📈 進捗, ✅ 達成, ⚠️ 注意点, 📝 まとめ）
   - 各セクションは2-3文以内。長い段落は書かない
   - **重要な数値や結論は太字にする**
   - 情報量が多い場合は箇条書き（短い項目）で整理する
   - 全体で3-5セクションを目安にし、冗長にしない
2. **アクション提案モード**（スケジュール作成・タスク操作等）→ テキストは2-4文のみ。見出し・箇条書き・番号リスト禁止。ツール呼び出しが本体。

短い質問には短く答える。

## アクション提案モードの出力ルール（最重要 — 厳守）

**テキスト出力のルール:**
- 方針・判断根拠を2-4文の平文で書く。それ以上書かない
- 見出し（##）、箇条書き（-）、番号リスト（1. 2. 3.）は使わない
- ツールで提案する内容（タイムブロック名、時間帯、タスク名等）をテキストに列挙しない
- UIがスケジュール表やアクション一覧を視覚的に表示するので、テキストで同じ内容を繰り返さない

**正しい例:**
行動パターンから17時・14時台が生産性ピークなので、Kensanリリース関連のタスクをそこに集中配置しています。引越し準備は集中力が不要な時間帯に。

**間違った例（これをやるな）:**
## スケジュール提案
以下のスケジュールを提案します:
1. 09:00-10:00 英語学習
2. 10:00-12:00 Kensan開発
3. 14:00-15:00 資格勉強
...

↑ ツール呼び出しと同じ情報をテキストに書いている。これは冗長。

**ツール呼び出しのルール:**
1. 読み取りツールでデータ収集（get_time_blocks, get_tasks 等）
2. テキストで方針を2-4文出力
3. **同じターンで**書き込みツールをまとめて呼ぶ（create_time_block × N 等）

**書き込みツールは「提案」であり即時実行ではない。**
ユーザーが承認するまで実行されない。UIがチェックボックス付きで表示し、個別に承認/却下できる。

**禁止事項:**
- テキストに提案内容を列挙する（ツール呼び出しと重複して冗長）
- 「作成しました」「設定しました」「登録します」等の完了形（承認前なので「提案します」が正しい）
- 「実行してよいですか？」「作りましょうか？」とテキストで確認する（UIが承認フローを担当する）
- 1つずつツールを呼ぶ（まとめて呼ぶこと）
- スケジュール作成の依頼に対してタスク作成だけを提案すること（タイムブロック配置が最終ゴール）

ユーザーが明らかに情報収集だけを求めている場合（「教えて」「見せて」）はアクション提案しない。

## 日本語の解釈ガイド

以下のような表現は新規作成ではなく、既存データの操作・参照を意味する：
- 「期限が厳しいタスク」→ 期限が近い既存タスクを検索
- 「来週の予定」→ 来週のタイムブロックを取得
- 「資格の進捗」→ 関連する目標・タスクの完了状況を確認
- 「終わったタスク」→ completed=true のタスクを取得

新規作成を示す表現：
- 「〜を作って」「〜を追加して」「〜を入れて」

相談・提案を示す表現（データ取得→分析→アクション提案のフローで対応）：
- 「〜を相談したい」「〜を見直したい」「〜を調整したい」「〜どうしよう」「〜についてアドバイス」

## ルール
- 日本語で応答する
- 書き込み操作はツール呼び出しで提案する。UIが承認フローを表示するので、テキストで「実行してよいですか？」と聞かない
- 読み取り操作は即実行してよい
- 日付は JST 基準。「今日」「明日」等は JST で解釈する
- スケジュール提案の日付範囲: 「今週」→ 今日〜今週の日曜日（{current_datetime}の週）。「来週」→ 次の月曜〜日曜。時期指定なし→ 今日〜今週日曜。**ユーザーが指定した期間を超えて提案しない**（「今週」と言われたら来週の日付を含めない）
- 曖昧な時間: 朝→08:00-09:00、昼→12:00-13:00、午後→14:00-15:00、夕方→17:00-18:00
- 単純な操作は短く、分析や振り返りの依頼には深く答える
- ユーザーにIDや技術的情報を聞かない。必要な情報はツールで取得する
- 意図が明確ならそのまま実行する。本当に曖昧な場合のみ短く確認する
"""

ALLOWED_TOOLS = [
    # Read tools
    "get_goals_and_milestones",
    "get_tasks",
    "get_time_blocks",
    "get_time_entries",
    "get_memos",
    "get_notes",
    "get_reviews",
    "get_review",
    "get_analytics_summary",
    "get_daily_summary",
    "get_user_memory",
    "get_user_facts",
    "get_recent_interactions",
    "semantic_search",
    "keyword_search",
    "hybrid_search",
    # External tools
    "web_search",
    "web_fetch",
    # Write tools
    "create_task",
    "update_task",
    "delete_task",
    "create_time_block",
    "update_time_block",
    "delete_time_block",
    "create_memo",
    "create_note",
    "update_note",
    "create_goal",
    "update_goal",
    "delete_goal",
    "create_milestone",
    "update_milestone",
    "delete_milestone",
    "add_user_fact",
    "generate_review",
]

# =========================================================================
# Dynamic Tool Selection
# =========================================================================

TOOL_GROUPS: dict[str, list[str]] = {
    "core": [  # 常に読み込む
        "get_tasks",
        "get_time_blocks",
        "get_time_entries",
        "get_memos",
    ],
    "planning": [  # 予定・スケジュール関連
        "create_time_block",
        "update_time_block",
        "delete_time_block",
    ],
    "task": [  # タスク作成・編集
        "create_task",
        "update_task",
        "delete_task",
    ],
    "goals_read": [  # 目標・マイルストーン（参照のみ）
        "get_goals_and_milestones",
    ],
    "goals_write": [  # 目標・マイルストーン（変更）
        "create_goal",
        "update_goal",
        "delete_goal",
        "create_milestone",
        "update_milestone",
        "delete_milestone",
    ],
    "notes_read": [  # ノート（参照のみ）
        "get_notes",
    ],
    "notes_write": [  # ノート・メモ（作成・編集）
        "create_note",
        "update_note",
        "create_memo",
    ],
    "analytics": [  # 分析・振り返り
        "get_analytics_summary",
        "get_daily_summary",
        "get_goals_and_milestones",
    ],
    "search": [  # 検索
        "semantic_search",
        "keyword_search",
        "hybrid_search",
        "reindex_notes",
    ],
    "review": [  # レビュー
        "get_reviews",
        "get_review",
        "generate_review",
    ],
    "memory": [  # ユーザー記憶
        "get_user_memory",
        "get_user_facts",
        "get_recent_interactions",
        "add_user_fact",
    ],
    "web": [  # Web検索・取得
        "web_search",
        "web_fetch",
    ],
    "patterns": [  # 行動パターン分析
        "get_user_patterns",
    ],
}

SITUATION_TOOL_GROUPS: dict[str, list[str]] = {
    # 明示指定された situation → 必要なグループを静的に定義
    # weekly: {weekly_summary} が VariableReplacer で既に埋め込まれるため analytics 不要
    "review": ["core", "review", "notes_read", "goals_read", "search", "patterns"],
    "daily_advice": ["core", "planning", "task", "goals_read", "analytics", "patterns"],
}

# フロントから渡された context キー → 除外するツール
# context にデータが含まれていれば、対応するツールを allowed_tools から除外し
# エージェントがツールで再取得するのを防ぐ
CONTEXT_EXCLUDES_TOOLS: dict[str, list[str]] = {
    "週間サマリー": ["get_analytics_summary", "get_daily_summary"],
    "日別稼働": ["get_time_entries"],
    "目標進捗": ["get_goals_and_milestones"],
    "タスク一覧": ["get_tasks"],
}

# システムプロンプトの変数 → 対応するツールを除外
# 変数でデータが注入済みなら、同じデータを返すツールは不要
VARIABLE_EXCLUDES_TOOLS: dict[str, list[str]] = {
    "pending_tasks": ["get_tasks"],
    "goal_progress": ["get_goals_and_milestones"],
    "today_schedule": ["get_time_blocks"],
    "today_entries": ["get_time_entries"],
    "user_patterns": ["get_user_patterns"],
    "yesterday_entries": ["get_time_entries"],
    "recent_learning_notes": ["get_notes"],
    "weekly_summary": ["get_analytics_summary", "get_daily_summary"],
}

# 参照系: 常にマッチするとreadグループを追加
INTENT_READ_PATTERNS: list[tuple[list[str], list[str]]] = [
    (["予定", "スケジュール", "タイムブロック"], ["goals_read"]),
    (["タスク", "やること", "TODO"], []),  # core に get_tasks 含む
    (["目標", "ゴール", "マイルストーン", "達成"], ["goals_read", "analytics"]),
    (["ノート", "メモ", "日記", "記録"], ["notes_read"]),
    (["分析", "進捗", "レポート", "サマリー", "振り返り"], ["analytics", "review"]),
    (["検索", "探して", "調べて", "どこ"], ["search"]),
    (["レビュー", "週次", "ウィークリー"], ["review", "analytics"]),
    (["ウェブ", "Web", "web", "ググって", "最新", "ニュース", "公式", "URL", "サイト", "ページ"], ["web"]),
]

# =========================================================================
# 書き込み意図の判定（語幹ベース）
# =========================================================================
# 従来の完全一致キーワードでは活用形の取りこぼしが多い
# （例: 「作って」は検出できても「作りたい」「作ろう」は検出できない）
#
# 動詞を語幹ベースでマッチさせることで、全活用形を網羅的にカバーする:
# 1. サ変動詞 → 名詞部分でマッチ（作成 → 作成して/作成したい/作成しよう 全てOK）
# 2. 一段動詞 → 語幹でマッチ（入れ → 入れて/入れたい/入れよう 全てOK）
# 3. 五段動詞 → 語幹＋活用行の正規表現（作[らりるれろっ] → 作って/作りたい/作ろう 全てOK）
# 4. その他 → 部分文字列マッチ

# サ変動詞: 名詞部分でマッチ（「〜する」の全活用形をカバー）
_SURU_VERB_STEMS: list[str] = [
    "作成", "追加", "登録", "変更", "更新", "修正", "編集",
    "削除", "取消", "取り消", "記録", "確保",
    "調整", "整理", "提案", "計画",
]

# 一段動詞: 語幹（「る」を除いた部分）でマッチ
_ICHIDAN_VERB_STEMS: list[str] = [
    "入れ",  # 入れる
    "決め",  # 決める
    "埋め",  # 埋める
    "立て",  # 立てる
]

# 五段動詞: 語幹＋活用行（子音行の全段をカバー）
_GODAN_VERB_PATTERNS: list[re.Pattern[str]] = [
    re.compile(r"作[らりるれろっ]"),   # 作る
    re.compile(r"つく[らりるれろっ]"),  # つくる
    re.compile(r"書[かきくけこい]"),   # 書く
    re.compile(r"消[さしすせそ]"),    # 消す
    re.compile(r"組[まみむめもん]"),   # 組む
]

# 活用形に依存しないキーワード
_OTHER_WRITE_KEYWORDS: list[str] = [
    "相談", "見直", "メモし",
]

# 書き込みキーワード × ドメインキーワード → writeグループ
INTENT_WRITE_PATTERNS: list[tuple[list[str], list[str]]] = [
    (["予定", "スケジュール", "タイムブロック"], ["planning"]),
    (["タスク", "やること", "TODO"], ["task"]),
    (["目標", "ゴール", "マイルストーン"], ["goals_write"]),
    (["ノート", "メモ", "日記", "記録"], ["notes_write"]),
]

# =========================================================================
# Deferred Write Tool Injection
# =========================================================================
# read ツール呼び出し自体が write 意図のシグナルになる。
# LLM が read ツールを呼んだ後、対応する write ツールを動的に追加する。

READ_TOOL_TO_WRITE_GROUPS: dict[str, list[str]] = {
    "get_tasks": ["task"],
    "get_time_blocks": ["planning"],
    "get_time_entries": ["planning"],
    "get_goals_and_milestones": ["goals_write"],
    "get_notes": ["notes_write"],
    "get_memos": ["notes_write"],
    "get_reviews": ["review"],
    "get_review": ["review"],
    "get_user_facts": ["memory"],
    "get_user_memory": ["memory"],
}


def get_deferred_write_tools(
    selected_tools: list[str],
    base_tools: list[str],
) -> list[str]:
    """selected ツールリストから、deferred で追加すべき write ツールを返す。

    read ツールが選択されていれば、対応する write グループの write ツールのみを返す。
    既に selected_tools に含まれている write ツールは除外される。
    base_tools（DB許可リスト）にないツールは除外される。

    Args:
        selected_tools: select_tools() で選択されたツールのリスト
        base_tools: DBで許可されたツールの上限リスト

    Returns:
        deferred で追加する write ツール名のリスト
    """
    deferred_groups: set[str] = set()
    for tool_name in selected_tools:
        groups = READ_TOOL_TO_WRITE_GROUPS.get(tool_name)
        if groups:
            deferred_groups.update(groups)

    if not deferred_groups:
        return []

    # グループからツール名に展開し、write ツールのみフィルタ
    deferred: set[str] = set()
    for group in deferred_groups:
        for tool_name in TOOL_GROUPS.get(group, []):
            if not is_readonly_tool(tool_name):
                deferred.add(tool_name)

    # 既に選択済みのツールは deferred から除外
    selected_set = set(selected_tools)
    deferred -= selected_set

    # base_tools（許可リスト）とのANDで返す
    base_set = set(base_tools)
    return [t for t in deferred if t in base_set]


def _has_write_intent(message: str) -> bool:
    """メッセージに書き込み意図があるかを判定する。

    動詞の語幹ベースでマッチするため、活用形のバリエーション
    （〜して/〜したい/〜しよう/〜しろ 等）を網羅的にカバーする。
    """
    if any(stem in message for stem in _SURU_VERB_STEMS):
        return True
    if any(stem in message for stem in _ICHIDAN_VERB_STEMS):
        return True
    if any(p.search(message) for p in _GODAN_VERB_PATTERNS):
        return True
    return any(kw in message for kw in _OTHER_WRITE_KEYWORDS)


def select_tools(
    message: str,
    base_tools: list[str],
    situation: str = "auto",
    context_keys: list[str] | None = None,
    prompt_variables: list[str] | None = None,
) -> list[str]:
    """situation とメッセージ意図からツールグループを選択し、必要なツールだけを返す。

    Args:
        message: ユーザーのメッセージテキスト
        base_tools: DBで許可されたツールのリスト（上限）
        situation: リクエストの situation（"auto" の場合はキーワードベースで選択）
        context_keys: フロントから渡された context のキー。対応ツールを除外する。
        prompt_variables: システムプロンプトに含まれる変数名。対応ツールを除外する。

    Returns:
        選択されたツールのリスト
    """
    selected_groups: set[str] = {"core"}  # 常に含む

    if situation in SITUATION_TOOL_GROUPS:
        # 明示 situation → 静的グループ
        selected_groups.update(SITUATION_TOOL_GROUPS[situation])
    else:
        # auto / chat → キーワードベースでグループ選択
        matched_read = False
        for keywords, groups in INTENT_READ_PATTERNS:
            if any(kw in message for kw in keywords):
                selected_groups.update(groups)
                matched_read = True

        # 書き込み意図が明確な場合、write グループも直接追加
        # （deferred injection だけだと read ツールが不要なケースで write ツールが解放されない）
        if _has_write_intent(message):
            for domain_keywords, write_groups in INTENT_WRITE_PATTERNS:
                if any(kw in message for kw in domain_keywords):
                    selected_groups.update(write_groups)

        # 読み書きどちらにもマッチなし → デフォルトセット（read only）
        if not matched_read and selected_groups == {"core"}:
            selected_groups.add("goals_read")

    # グループからツール名リストに展開
    selected: set[str] = set()
    for group in selected_groups:
        selected.update(TOOL_GROUPS.get(group, []))

    # context で提供済みのデータに対応するツールを除外
    if context_keys:
        for key in context_keys:
            for tool_name in CONTEXT_EXCLUDES_TOOLS.get(key, []):
                selected.discard(tool_name)

    # プロンプト変数で注入済みのデータに対応するツールを除外
    if prompt_variables:
        for var in prompt_variables:
            for tool_name in VARIABLE_EXCLUDES_TOOLS.get(var, []):
                selected.discard(tool_name)

    # base_tools（許可リスト）とのANDで返す
    return [t for t in base_tools if t in selected]


# =========================================================================
# Auto Model Selection (Haiku / Sonnet)
# =========================================================================

COMPLEX_PATTERNS: list[str] = [
    "レビュー", "週次", "振り返り", "分析", "進捗", "レポート",
    "計画", "プランニング", "相談", "アドバイス",
    "なぜ", "どうして", "原因", "改善",
]


def select_model(message: str, situation: str = "auto") -> str:
    """メッセージの複雑さに基づいてモデルを自動選択。

    review/daily_advice や複雑なキーワードを含むメッセージには Sonnet、
    それ以外の軽い質問には Haiku を返す。

    Args:
        message: ユーザーのメッセージテキスト
        situation: リクエストの situation

    Returns:
        モデル名の文字列
    """
    settings = get_settings()

    # review/daily_advice は常に Sonnet（品質重視）
    if situation in ("review", "daily_advice"):
        return settings.anthropic_model

    # 複雑なキーワードが含まれていれば Sonnet
    for keyword in COMPLEX_PATTERNS:
        if keyword in message:
            return settings.anthropic_model

    # メッセージが長い（100文字以上）なら Sonnet
    if len(message) > 100:
        return settings.anthropic_model

    # それ以外は Haiku（高速）
    return settings.anthropic_fast_model
