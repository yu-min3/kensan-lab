"""
DuckDBインタラクティブクエリ用ヘルパー
PyIceberg経由でIcebergテーブルをDuckDBに登録し、SQLで分析可能にする。

使い方:
    uv run python queries/query.py          # 全テーブル登録してREPL起動
    uv run python queries/query.py summary  # サマリーを表示して終了
"""

import sys

import duckdb

from catalog.config import get_catalog

# Iceberg → DuckDB テーブルマッピング
TABLES = {
    # Bronze
    "bronze_time_entries": "bronze.time_entries_raw",
    "bronze_tasks": "bronze.tasks_raw",
    "bronze_notes": "bronze.notes_raw",
    "bronze_ai_interactions": "bronze.ai_interactions_raw",
    "bronze_ai_facts": "bronze.ai_facts_raw",
    "bronze_ai_reviews": "bronze.ai_reviews_raw",
    "bronze_ai_contexts": "bronze.ai_contexts_raw",
    "bronze_external_tool_results": "bronze.external_tool_results_raw",
    # Silver
    "silver_time_entries": "silver.time_entries",
    "silver_tasks": "silver.tasks",
    "silver_notes": "silver.notes",
    "silver_ai_interactions": "silver.ai_interactions",
    "silver_ai_token_usage": "silver.ai_token_usage",
    "silver_ai_facts": "silver.ai_facts",
    "silver_ai_reviews": "silver.ai_reviews",
    # Gold
    "gold_weekly_summary": "gold.weekly_summary",
    "gold_goal_progress": "gold.goal_progress",
    "gold_ai_usage_weekly": "gold.ai_usage_weekly",
    "gold_ai_quality_weekly": "gold.ai_quality_weekly",
}


def setup(con: duckdb.DuckDBPyConnection):
    """全Icebergテーブルを DuckDB に登録"""
    catalog = get_catalog()
    for duckdb_name, iceberg_name in TABLES.items():
        arrow_table = catalog.load_table(iceberg_name).scan().to_arrow()
        con.register(duckdb_name, arrow_table)
        print(f"  Registered: {duckdb_name} ({len(arrow_table)} rows)")


def show_summary(con: duckdb.DuckDBPyConnection):
    """全テーブルのサマリーを表示"""
    print("\n=== Gold: Weekly Summary ===")
    print(con.sql("""
        SELECT week_start,
               total_minutes / 60 AS total_hours,
               task_count,
               completed_task_count,
               note_count
        FROM gold_weekly_summary
        ORDER BY week_start
    """))

    print("\n=== Gold: Goal Progress ===")
    print(con.sql("""
        SELECT goal_name,
               week_start,
               total_minutes / 60 AS hours,
               entry_count
        FROM gold_goal_progress
        ORDER BY goal_name, week_start
    """))

    print("\n=== Silver: Time by Goal ===")
    print(con.sql("""
        SELECT goal_name,
               count(*) AS entries,
               sum(duration_minutes) / 60 AS total_hours,
               round(avg(duration_minutes), 1) AS avg_minutes
        FROM silver_time_entries
        GROUP BY goal_name
        ORDER BY total_hours DESC
    """))

    # AI tables
    ai_ctx_count = con.sql("SELECT count(*) AS cnt FROM bronze_ai_contexts").fetchone()[0]
    ai_review_count = con.sql("SELECT count(*) AS cnt FROM bronze_ai_reviews").fetchone()[0]
    if ai_ctx_count > 0 or ai_review_count > 0:
        print("\n=== Bronze: AI Contexts ===")
        print(con.sql("""
            SELECT name, situation, is_active, max_turns, temperature
            FROM bronze_ai_contexts
            ORDER BY situation, name
        """))

        print("\n=== Bronze: AI Reviews ===")
        print(con.sql("""
            SELECT user_id, week_start, week_end,
                   length(summary) AS summary_len,
                   tokens_input, tokens_output
            FROM bronze_ai_reviews
            ORDER BY week_start
        """))

    ai_quality_count = con.sql("SELECT count(*) AS cnt FROM gold_ai_quality_weekly").fetchone()[0]
    if ai_quality_count > 0:
        print("\n=== Gold: AI Quality Weekly ===")
        print(con.sql("""
            SELECT user_id, week_start, rated_count,
                   round(avg_rating, 2) AS avg_rating,
                   fact_count, review_generated
            FROM gold_ai_quality_weekly
            ORDER BY week_start
        """))


def main():
    con = duckdb.connect()

    print("Loading Iceberg tables into DuckDB...")
    setup(con)
    print()

    if len(sys.argv) > 1 and sys.argv[1] == "summary":
        show_summary(con)
        return

    # テーブル一覧を表示
    print("Available tables:")
    for name in TABLES:
        print(f"  - {name}")
    print()
    print("Starting DuckDB interactive shell.")
    print("Type SQL queries, or '.quit' to exit.\n")

    # インタラクティブループ
    while True:
        try:
            query = input("D> ").strip()
        except (EOFError, KeyboardInterrupt):
            print("\nBye.")
            break

        if not query:
            continue
        if query.lower() in (".quit", ".exit", "quit", "exit"):
            print("Bye.")
            break
        if query.lower() == ".tables":
            for name in TABLES:
                print(f"  {name}")
            continue

        try:
            result = con.sql(query)
            print(result)
        except Exception as e:
            print(f"Error: {e}")


if __name__ == "__main__":
    main()
