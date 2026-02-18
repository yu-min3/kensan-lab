-- ===========================================
-- DuckDB アドホッククエリ例
-- 使い方: make query で DuckDB REPL を起動
-- ===========================================

-- ※ DuckDB で Iceberg テーブルを読むには iceberg 拡張が必要
-- INSTALL iceberg;
-- LOAD iceberg;

-- ----- Silver層: 時間エントリ -----

-- ゴール別の合計時間
SELECT goal_name,
       sum(duration_minutes) / 60.0 AS total_hours,
       count(*) AS entries
FROM iceberg_scan('s3://kensan-lakehouse/silver/time_entries/')
GROUP BY goal_name
ORDER BY total_hours DESC;

-- 日別の作業時間
SELECT date,
       sum(duration_minutes) AS total_minutes,
       count(*) AS entries
FROM iceberg_scan('s3://kensan-lakehouse/silver/time_entries/')
GROUP BY date
ORDER BY date DESC
LIMIT 30;

-- ----- Silver層: タスク -----

-- 完了率
SELECT completed,
       count(*) AS count
FROM iceberg_scan('s3://kensan-lakehouse/silver/tasks/')
GROUP BY completed;

-- サブタスクの割合
SELECT is_subtask,
       count(*) AS count
FROM iceberg_scan('s3://kensan-lakehouse/silver/tasks/')
GROUP BY is_subtask;

-- ----- Silver層: ノート -----

-- タイプ別のノート数
SELECT type,
       count(*) AS count,
       avg(content_length) AS avg_length
FROM iceberg_scan('s3://kensan-lakehouse/silver/notes/')
GROUP BY type;

-- ----- Gold層: 週次サマリー -----

-- 直近の週次サマリー
SELECT week_start,
       total_minutes / 60.0 AS total_hours,
       task_count,
       completed_task_count,
       note_count
FROM iceberg_scan('s3://kensan-lakehouse/gold/weekly_summary/')
ORDER BY week_start DESC
LIMIT 10;

-- ----- Gold層: ゴール進捗 -----

-- ゴール別の週次推移
SELECT goal_name,
       week_start,
       total_minutes / 60.0 AS hours,
       entry_count
FROM iceberg_scan('s3://kensan-lakehouse/gold/goal_progress/')
ORDER BY goal_name, week_start;
