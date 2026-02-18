-- Add 'mindmap' to note_contents.content_type CHECK constraint
ALTER TABLE note_contents
    DROP CONSTRAINT IF EXISTS note_contents_content_type_check;

ALTER TABLE note_contents
    ADD CONSTRAINT note_contents_content_type_check
    CHECK (content_type IN ('markdown', 'drawio', 'image', 'pdf', 'code', 'mindmap'));
