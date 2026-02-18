"""Agent evaluator for actively testing and assessing AI agent quality.

Unlike PromptEvaluator (which analyzes historical conversations), this module
actively sends test messages to each agent situation and evaluates real-time
responses against Kensan-specific quality criteria.

Flow:
1. Load test scenarios for each situation (chat, daily_advice, review)
2. Send each test message to the agent via internal API
3. Evaluate the response using the evaluator LLM with EVALUATOR_SYSTEM_PROMPT
4. Aggregate results into a structured report
"""

from __future__ import annotations

import json
import logging
from typing import Any
from uuid import UUID

from kensan_ai.config import get_settings
from kensan_ai.lib.ai_provider import LLMClient
from kensan_ai.lib.llm_utils import extract_json_from_response

logger = logging.getLogger(__name__)

# =========================================================================
# Evaluator System Prompt
# =========================================================================

EVALUATOR_SYSTEM_PROMPT = """あなたは Kensan アプリのAIエージェント品質評価の専門家です。
Kensan はエンジニア向けパーソナル生産性アプリで、時間管理・タスク管理・目標管理・学習記録・AI週次レビューを提供します。

あなたの役割は、各AIエージェント（chat, daily_advice, review）の応答を多角的に評価し、改善点を具体的に指摘することです。

---

## 1. Kensan の利用コンテキスト

- **ユーザー像**: ソフトウェアエンジニア。日々のスケジュール管理、技術学習の記録、目標達成のトラッキングに Kensan を使う
- **フロントエンド**: React SPA。右サイドパネルにチャットUI。タイムブロックはタイムライン形式で視覚表示。アクション提案はチェックボックス付きで個別承認/却下可能
- **AIの価値**: 単なる CRUD 代行ではなく、ユーザーの行動パターン・目標進捗・過去の傾向を踏まえた「洞察」と「具体的提案」を提供すること

---

## 2. 評価対象の Situation

### 2.1 chat（汎用チャット）
- **目的**: ユーザーの自由な要求に応える。予定作成・タスク管理・分析・相談・検索など
- **期待行動**:
  - 意図を正しく判定（参照 / 作成 / 相談）し、最小限のツール呼び出しで対応
  - 上位コンテキスト（目標進捗・行動パターン・未完了タスク）がシステムプロンプトに注入済みなので、それを活用してから不足分だけツールで取得
  - 「予定を組んで」→ 最終的に create_time_block まで提案する（create_task だけで終わらない）
  - 「相談したい」→ データ取得 → 分析 → アクション提案の一連フロー

### 2.2 daily_advice（デイリーアドバイス）
- **目的**: 今日の計画を最適化し、生産的な1日をサポート
- **期待行動**:
  - 今日の予定・タスク・目標進捗を俯瞰的に分析
  - 優先順位の提案、時間配分の最適化
  - ユーザーの生産性パターン（ピーク時間帯等）を考慮
  - 潜在リスク（タスク過多、期限切れ間近）を指摘

### 2.3 review（週次レビュー）
- **目的**: 1週間を振り返り、次週への具体的アクションを提示
- **期待行動**:
  - 定量データ（稼働時間、目標進捗、タスク完了率）に基づく分析
  - 良かった点と改善点のバランス
  - 長期目標との接続（今週の動きが目標達成にどう寄与するか）
  - 次週への具体的アドバイス

---

## 3. 評価軸（全 situation 共通）

各応答を以下の 6 軸で 1〜5 点で評価してください。

### 3.1 フロントエンド適合性 (frontend_fit)
フロントエンドUIで正しく・効果的に表示されるか。

**5点の基準:**
- アクション提案モード時: テキストは2-4文の平文。見出し・箇条書き・番号リストなし。ツール呼び出しが本体
- 情報提供モード時: 見出し（##, ###）で構成。構造的で読みやすい
- ツール呼び出し内容をテキストで繰り返していない（UIがタイムライン/チェックリストで可視化するため冗長になる）
- 書き込みツールについて「作成しました」「登録します」等の完了表現を使っていない（承認前なので「提案します」が正しい）
- 「実行してよいですか？」等のテキスト確認をしていない（UIが承認フローを担当）
- 複数の書き込みツールを1ターンにまとめて呼んでいる

**1点の基準:**
- アクション提案時にテキストで全内容を列挙（ツールと重複）
- 「以下を作成しました」等の完了表現
- 「作りましょうか？」のようなテキスト確認
- ツールを1つずつ別ターンで呼ぶ
- 情報提供時に構造がなく読みにくい

### 3.2 洞察の深さ (insight_depth)
表面的な情報伝達を超えた、ユーザーにとって価値ある洞察があるか。

**5点の基準:**
- ユーザーの行動パターン・目標進捗・過去の傾向を横断的に分析
- ユーザーが気づいていない潜在リスクや機会を指摘
- データに基づいた根拠のある提案（「生産性ピークの14時台にXを配置」等）
- 複数の情報ソースを結びつけた統合的な見解

**1点の基準:**
- 単にデータをそのまま読み上げるだけ
- 「頑張りましょう」等の一般的な激励のみ
- コンテキスト（目標、パターン等）を全く活用していない

### 3.3 実行可能性 (actionability)
提案が具体的で、ユーザーがすぐに実行に移せるか。

**5点の基準:**
- create_time_block / create_task 等の具体的なツール呼び出しで提案
- 時間帯・所要時間・タスク名が具体的
- ユーザーの既存スケジュールとの整合性を確認済み（空きスロットに配置）
- 必要なら前提タスクも同時に提案

**1点の基準:**
- テキストで「～するといいでしょう」と言うだけでツール提案なし
- 具体性がない（「時間を見つけてやりましょう」）
- 既存スケジュールとの衝突を考慮していない

### 3.4 効率性 (efficiency)
最小限のターン・ツール呼び出しで目的を達成しているか。

**5点の基準:**
- システムプロンプトに注入済みのデータを活用し、不要なツール呼び出しをしていない
- 複数ツールを1ターンにまとめて呼んでいる
- 短い質問には短く答えている
- 読み取りツールで情報収集 → テキストで方針 → 書き込みツールで提案、が1回のやりとりで完結

**1点の基準:**
- 注入済みデータを無視して同じ情報をツールで再取得
- 1つずつ質問を返して何ターンもかかる
- 簡単な質問に長文で返す
- 「もう少し詳しく教えてください」と不要な確認

### 3.5 日本語自然さ (japanese_quality)
自然で読みやすい日本語か。エンジニアに適したトーンか。

**5点の基準:**
- エンジニアに対する適切な敬体（ですます調、過度にカジュアルでも硬すぎもしない）
- 技術用語の適切な使用
- 簡潔でテンポの良い文章
- 日本語の慣用表現を正しく解釈（「立てて」→作成、「見直したい」→相談、等）

**1点の基準:**
- 機械翻訳調、不自然な助詞
- 冗長な前置き・枕詞
- エンジニアに対して子供扱いするような表現
- 日本語の意図を誤解（「期限が厳しいタスク」を新規作成と解釈する等）

### 3.6 ユーザー価値 (user_value)
総合的に、Kensan ユーザーのパーソナル生産性向上に寄与する応答か。

**5点の基準:**
- AIがいなければ見落としていた気づきがある
- 提案を承認するだけで具体的に次のアクションに進める
- ユーザーの長期的な目標達成・生産性パターン改善につながる
- 「このAIは自分のことを理解している」と感じられる

**1点の基準:**
- 手動で操作した方が速い程度の応答
- 的外れな提案
- ユーザーの状況を無視した汎用的な回答

---

## 4. Situation 別の追加チェック項目

### 4.1 chat 追加チェック
- [ ] 意図判定: 参照（「教えて」「見せて」）→ 情報提供モード / 作成（「作って」「追加して」）→ アクション提案モード / 相談（「相談したい」「調整して」）→ データ取得→分析→アクション提案
- [ ] スケジュール作成依頼で create_task だけ提案して create_time_block を提案していない → NG
- [ ] 既にシステムプロンプトに含まれるデータ（{pending_tasks}, {goal_progress}等）を確認するためだけにツールを呼んでいない
- [ ] 曖昧な時間表現の解釈: 朝→08:00-09:00、昼→12:00-13:00、午後→14:00-15:00、夕方→17:00-18:00
- [ ] ユーザーにIDや技術的パラメータを尋ねていない

### 4.2 daily_advice 追加チェック
- [ ] 今日の予定を把握した上でアドバイスしている
- [ ] タスクの優先度（期限の近さ、目標との関連）を考慮
- [ ] ユーザーの生産性パターン（ピーク時間帯）を反映したスケジュール提案
- [ ] 過去の傾向（計画精度、慢性的な遅延等）を踏まえた現実的な提案

### 4.3 review 追加チェック
- [ ] 定量データ（稼働時間、完了タスク数、目標進捗率等）を引用
- [ ] 良かった点を具体的に認めている（モチベーション維持）
- [ ] 改善点が建設的で具体的（「頑張りましょう」ではなく具体的なアクション）
- [ ] 次週への接続がある（来週何をすべきか）
- [ ] 長期目標との整合性に言及

---

## 5. 出力形式

以下の JSON 形式で評価結果を出力してください。JSON 以外のテキストは不要です。

```json
{
  "situation": "chat | daily_advice | review",
  "test_message": "テストに使った入力メッセージ",
  "scores": {
    "frontend_fit": { "score": 1-5, "reason": "根拠" },
    "insight_depth": { "score": 1-5, "reason": "根拠" },
    "actionability": { "score": 1-5, "reason": "根拠" },
    "efficiency": { "score": 1-5, "reason": "根拠" },
    "japanese_quality": { "score": 1-5, "reason": "根拠" },
    "user_value": { "score": 1-5, "reason": "根拠" }
  },
  "checklist_violations": [
    "違反した追加チェック項目の説明"
  ],
  "overall_assessment": "総合評価コメント（2-3文）",
  "improvement_suggestions": [
    {
      "priority": "high | medium | low",
      "target": "system_prompt | tool_selection | response_format | other",
      "suggestion": "具体的な改善提案",
      "example_before": "現在の問題のある出力例（該当部分）",
      "example_after": "改善後の期待出力例"
    }
  ]
}
```
"""

# =========================================================================
# Test Scenarios
# =========================================================================

TEST_SCENARIOS: dict[str, list[dict[str, Any]]] = {
    "chat": [
        # --- 意図判定: 参照系 ---
        {
            "id": "chat_ref_01",
            "message": "来週の予定を教えて",
            "intent": "reference",
            "expected_tools": ["get_time_blocks"],
            "expected_mode": "information",
            "description": "来週のタイムブロック取得。情報提供モードで構造的に表示すべき",
        },
        {
            "id": "chat_ref_02",
            "message": "目標の進捗どうなってる？",
            "intent": "reference",
            "expected_tools": [],  # goal_progress はシステムプロンプトに注入済み
            "expected_mode": "information",
            "description": "注入済みデータで回答可能。不要なツール呼び出しをしないか確認",
        },
        {
            "id": "chat_ref_03",
            "message": "期限が近いタスクある？",
            "intent": "reference",
            "expected_tools": [],  # pending_tasks はシステムプロンプトに注入済み
            "expected_mode": "information",
            "description": "注入済みの pending_tasks で回答可能。「作成」と誤解しないか確認",
        },

        # --- 意図判定: 作成系 ---
        {
            "id": "chat_create_01",
            "message": "明日の午前中にKensan開発のタイムブロックを入れて",
            "intent": "create",
            "expected_tools": ["get_time_blocks", "create_time_block"],
            "expected_mode": "action_proposal",
            "description": "空きスロット確認 → タイムブロック提案。テキストは簡潔に。",
        },
        {
            "id": "chat_create_02",
            "message": "「API設計を完了する」というタスクを作って",
            "intent": "create",
            "expected_tools": ["create_task"],
            "expected_mode": "action_proposal",
            "description": "明示的なタスク作成。シンプルに提案すべき。",
        },
        {
            "id": "chat_create_03",
            "message": "来週のスケジュールを組んで",
            "intent": "create",
            "expected_tools": ["get_time_blocks", "get_tasks", "create_time_block"],
            "expected_mode": "action_proposal",
            "description": "最重要テスト。予定取得→タスク確認→タイムブロック群をまとめて提案。"
            "create_task だけで終わらないか、テキストで列挙しないか、を確認",
        },

        # --- 意図判定: 相談系 ---
        {
            "id": "chat_consult_01",
            "message": "今週のスケジュールを見直したい",
            "intent": "consult",
            "expected_tools": ["get_time_blocks"],
            "expected_mode": "action_proposal",
            "description": "現状分析→問題点指摘→調整提案のフロー。"
            "分析テキスト＋update/delete/create の提案",
        },
        {
            "id": "chat_consult_02",
            "message": "勉強の計画について相談したい",
            "intent": "consult",
            "expected_tools": ["get_time_blocks", "get_tasks"],
            "expected_mode": "action_proposal",
            "description": "目標・パターンを踏まえた学習計画の提案。洞察の深さが試される",
        },

        # --- エッジケース ---
        {
            "id": "chat_edge_01",
            "message": "Rustの所有権について教えて",
            "intent": "general_question",
            "expected_tools": [],
            "expected_mode": "information",
            "description": "Kensan のデータに無関係な一般質問。ツール不要で回答すべき。"
            "web_search を使うのは許容だが必須ではない",
        },
        {
            "id": "chat_edge_02",
            "message": "朝に英語、昼すぎにKensan、夕方に読書の予定を入れて",
            "intent": "create",
            "expected_tools": ["get_time_blocks", "create_time_block"],
            "expected_mode": "action_proposal",
            "description": "曖昧な時間表現3つ。朝→08:00-09:00、昼すぎ→13:00-14:00あたり、"
            "夕方→17:00-18:00 の解釈確認。3つの create_time_block をまとめて提案",
        },
        {
            "id": "chat_edge_03",
            "message": "うーん",
            "intent": "ambiguous",
            "expected_tools": [],
            "expected_mode": "information",
            "description": "意図不明の短い入力。短く返すか、文脈から推測して応答すべき。"
            "長文で返したり不要なツールを呼ばないか確認",
        },
    ],

    "daily_advice": [
        {
            "id": "daily_01",
            "message": "今日のアドバイスをください",
            "description": "標準的なデイリーアドバイスリクエスト。"
            "今日の予定・タスク・パターンを踏まえた具体的提案が必要",
        },
        {
            "id": "daily_02",
            "message": "今日やる気が出ない",
            "description": "モチベーション低下時の対応。共感しつつ、小さな一歩を具体的に提案。"
            "スケジュール調整（負荷軽減）の提案があるとベスト",
        },
        {
            "id": "daily_03",
            "message": "今日タスクが多すぎる気がする",
            "description": "タスク過多の相談。優先順位付け・リスケジュールの提案。"
            "定量データ（タスク数、推定所要時間等）に基づくべき",
        },
    ],

    "review": [
        {
            "id": "review_01",
            "message": "今週の振り返りをお願い",
            "description": "標準的な週次レビュー。稼働時間・目標進捗・完了タスクの"
            "定量分析＋次週への提案が必要",
        },
        {
            "id": "review_02",
            "message": "今週はあまり進まなかった気がする",
            "description": "ネガティブな自己評価。データで客観的に振り返り、"
            "良かった点も指摘。改善は建設的に。",
        },
    ],
}


# =========================================================================
# Agent Evaluator Class
# =========================================================================

class AgentEvaluator:
    """Actively tests AI agents and evaluates their responses."""

    def __init__(self):
        self.llm = LLMClient()

    async def evaluate_response(
        self,
        situation: str,
        test_message: str,
        agent_response: str,
        tool_calls: list[dict[str, Any]] | None = None,
        action_proposals: list[dict[str, Any]] | None = None,
        system_prompt_used: str | None = None,
    ) -> dict[str, Any]:
        """Evaluate a single agent response using the evaluator LLM.

        Args:
            situation: The agent situation (chat, daily_advice, review)
            test_message: The test message sent to the agent
            agent_response: The agent's text response
            tool_calls: List of tools the agent called (readonly)
            action_proposals: List of write-tool proposals the agent made
            system_prompt_used: The system prompt that was used (optional, for context)

        Returns:
            Structured evaluation result (parsed from evaluator LLM output)
        """
        # Build the evaluation input
        eval_input = self._build_evaluation_input(
            situation=situation,
            test_message=test_message,
            agent_response=agent_response,
            tool_calls=tool_calls,
            action_proposals=action_proposals,
            system_prompt_used=system_prompt_used,
        )

        try:
            raw_output = await self._call_evaluator_llm(eval_input)
            return self._parse_evaluation_output(raw_output)
        except Exception as e:
            logger.error("Evaluation failed: %s", e)
            return {
                "error": str(e),
                "scores": {},
                "improvement_suggestions": [],
            }

    async def run_scenario(
        self,
        scenario: dict[str, Any],
        agent_caller: Any,  # Callable that sends message and returns response
    ) -> dict[str, Any]:
        """Run a single test scenario against an agent.

        Args:
            scenario: A test scenario dict from TEST_SCENARIOS
            agent_caller: Async callable(message, situation) -> (text, tool_calls, proposals)

        Returns:
            Evaluation result with scenario metadata
        """
        situation = scenario.get("situation", "chat")
        message = scenario["message"]

        text, tool_calls, proposals = await agent_caller(message, situation)

        evaluation = await self.evaluate_response(
            situation=situation,
            test_message=message,
            agent_response=text,
            tool_calls=tool_calls,
            action_proposals=proposals,
        )

        return {
            "scenario_id": scenario["id"],
            "scenario_description": scenario["description"],
            **evaluation,
        }

    async def run_all_scenarios(
        self,
        agent_caller: Any,
        situations: list[str] | None = None,
    ) -> dict[str, Any]:
        """Run all test scenarios and aggregate results.

        Args:
            agent_caller: Async callable(message, situation) -> (text, tool_calls, proposals)
            situations: Optional list of situations to test. Defaults to all.

        Returns:
            Aggregated evaluation report
        """
        target_situations = situations or list(TEST_SCENARIOS.keys())
        all_results: list[dict[str, Any]] = []

        for sit in target_situations:
            scenarios = TEST_SCENARIOS.get(sit, [])
            for scenario in scenarios:
                scenario_with_situation = {**scenario, "situation": sit}
                result = await self.run_scenario(scenario_with_situation, agent_caller)
                all_results.append(result)

        return self._aggregate_results(all_results)

    def _build_evaluation_input(
        self,
        situation: str,
        test_message: str,
        agent_response: str,
        tool_calls: list[dict[str, Any]] | None,
        action_proposals: list[dict[str, Any]] | None,
        system_prompt_used: str | None,
    ) -> str:
        """Build the evaluation input message for the evaluator LLM."""
        parts = [
            f"## 評価対象\n- Situation: {situation}\n- テストメッセージ: {test_message}",
        ]

        if system_prompt_used:
            # Truncate to avoid token overflow
            truncated = system_prompt_used[:2000]
            parts.append(f"\n## 使用されたシステムプロンプト（抜粋）\n```\n{truncated}\n```")

        parts.append(f"\n## エージェントのテキスト応答\n```\n{agent_response}\n```")

        if tool_calls:
            tool_summary = "\n".join(
                f"- {tc.get('name', 'unknown')}: {json.dumps(tc.get('input', {}), ensure_ascii=False)[:200]}"
                for tc in tool_calls
            )
            parts.append(f"\n## 呼び出された読み取りツール\n{tool_summary}")

        if action_proposals:
            proposal_summary = "\n".join(
                f"- {ap.get('tool_name', 'unknown')}: {json.dumps(ap.get('input', {}), ensure_ascii=False)[:200]}"
                for ap in action_proposals
            )
            parts.append(f"\n## 提案された書き込みアクション\n{proposal_summary}")
        else:
            parts.append("\n## 提案された書き込みアクション\nなし")

        parts.append("\n上記を評価してください。")

        return "\n".join(parts)

    async def _call_evaluator_llm(self, evaluation_input: str) -> str:
        """Call the evaluator LLM with the system prompt and evaluation input."""
        return await self.llm.generate(
            evaluation_input,
            max_tokens=4096,
            system=EVALUATOR_SYSTEM_PROMPT,
        )

    def _parse_evaluation_output(self, raw_output: str) -> dict[str, Any]:
        """Parse the evaluator LLM's JSON output."""
        try:
            return extract_json_from_response(raw_output or "{}")
        except json.JSONDecodeError as e:
            logger.error("Failed to parse evaluation output: %s", e)
            return {
                "parse_error": str(e),
                "raw_output": raw_output[:500],
                "scores": {},
                "improvement_suggestions": [],
            }

    def _aggregate_results(
        self, results: list[dict[str, Any]]
    ) -> dict[str, Any]:
        """Aggregate individual evaluation results into a summary report."""
        # Per-dimension averages
        dimensions = [
            "frontend_fit",
            "insight_depth",
            "actionability",
            "efficiency",
            "japanese_quality",
            "user_value",
        ]

        dim_scores: dict[str, list[float]] = {d: [] for d in dimensions}
        all_violations: list[str] = []
        all_suggestions: list[dict[str, Any]] = []
        per_situation: dict[str, list[dict[str, Any]]] = {}

        for result in results:
            scores = result.get("scores", {})
            for dim in dimensions:
                if dim in scores and isinstance(scores[dim], dict):
                    score = scores[dim].get("score")
                    if isinstance(score, (int, float)):
                        dim_scores[dim].append(score)

            violations = result.get("checklist_violations", [])
            if isinstance(violations, list):
                all_violations.extend(violations)

            suggestions = result.get("improvement_suggestions", [])
            if isinstance(suggestions, list):
                all_suggestions.extend(suggestions)

            sit = result.get("situation", "unknown")
            per_situation.setdefault(sit, []).append(result)

        # Calculate averages
        avg_scores = {}
        for dim, scores_list in dim_scores.items():
            if scores_list:
                avg_scores[dim] = round(sum(scores_list) / len(scores_list), 2)

        overall_avg = (
            round(sum(avg_scores.values()) / len(avg_scores), 2)
            if avg_scores
            else None
        )

        # Deduplicate and prioritize suggestions
        high_priority = [s for s in all_suggestions if s.get("priority") == "high"]
        medium_priority = [s for s in all_suggestions if s.get("priority") == "medium"]

        return {
            "total_scenarios": len(results),
            "average_scores": avg_scores,
            "overall_average": overall_avg,
            "checklist_violations": list(set(all_violations)),
            "top_suggestions": (high_priority + medium_priority)[:10],
            "per_situation_results": {
                sit: {
                    "scenario_count": len(sit_results),
                    "details": sit_results,
                }
                for sit, sit_results in per_situation.items()
            },
        }
