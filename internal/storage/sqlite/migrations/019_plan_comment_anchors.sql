-- +goose Up
-- Add quoted-text-range anchors, edit timestamps, and resolve state to plan comments.
ALTER TABLE plan_comments ADD COLUMN line_start INTEGER;
ALTER TABLE plan_comments ADD COLUMN line_end INTEGER;
ALTER TABLE plan_comments ADD COLUMN quoted_text TEXT;
ALTER TABLE plan_comments ADD COLUMN occurrence INTEGER;
ALTER TABLE plan_comments ADD COLUMN heading_slug TEXT;
ALTER TABLE plan_comments ADD COLUMN context_before TEXT;
ALTER TABLE plan_comments ADD COLUMN context_after TEXT;
ALTER TABLE plan_comments ADD COLUMN updated_at TIMESTAMP;
ALTER TABLE plan_comments ADD COLUMN resolved_at TIMESTAMP;

-- +goose Down
ALTER TABLE plan_comments DROP COLUMN line_start;
ALTER TABLE plan_comments DROP COLUMN line_end;
ALTER TABLE plan_comments DROP COLUMN quoted_text;
ALTER TABLE plan_comments DROP COLUMN occurrence;
ALTER TABLE plan_comments DROP COLUMN heading_slug;
ALTER TABLE plan_comments DROP COLUMN context_before;
ALTER TABLE plan_comments DROP COLUMN context_after;
ALTER TABLE plan_comments DROP COLUMN updated_at;
ALTER TABLE plan_comments DROP COLUMN resolved_at;
