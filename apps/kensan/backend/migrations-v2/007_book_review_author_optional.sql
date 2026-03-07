-- ============================================================================
-- 007_book_review_author_optional.sql
-- Make the "author" metadata field optional for book_review note type
-- ============================================================================

BEGIN;

UPDATE note_types
SET metadata_schema = '[
    {"key": "author", "label": "著者", "labelEn": "Author", "type": "string", "required": false, "constraints": {}},
    {"key": "rating", "label": "評価", "labelEn": "Rating", "type": "integer", "required": false, "constraints": {"min": 1, "max": 5}},
    {"key": "isbn", "label": "ISBN", "labelEn": "ISBN", "type": "string", "required": false, "constraints": {}},
    {"key": "publisher", "label": "出版社", "labelEn": "Publisher", "type": "string", "required": false, "constraints": {}},
    {"key": "finished_date", "label": "読了日", "labelEn": "Finished Date", "type": "date", "required": false, "constraints": {}},
    {"key": "category", "label": "カテゴリ", "labelEn": "Category", "type": "enum", "required": false, "constraints": {"values": ["技術書", "ビジネス", "自己啓発", "小説", "その他"]}}
]'::jsonb
WHERE slug = 'book_review';

COMMIT;
