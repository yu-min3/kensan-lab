CREATE TABLE IF NOT EXISTS {{table}} (
 record_id VARCHAR,
 app_id VARCHAR,
 capture_id VARCHAR,
 raw_object_id VARCHAR,
 raw_sha256 VARCHAR,
 source_uri VARCHAR,
 source_group VARCHAR,
 schema_version VARCHAR,
 transform_version VARCHAR,
 adapter_sha256 VARCHAR,
 prepared_text VARCHAR,
 text_bytes BIGINT,
 text_truncated BOOLEAN,
 content_json VARCHAR,
 recipe STRUCT(title VARCHAR, servings VARCHAR, ingredients VARCHAR[], steps VARCHAR[]),
 output_schema VARCHAR,
 warnings VARCHAR[]
)
