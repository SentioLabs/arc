-- +goose Up
-- Migration 014: Simplify plans to ephemeral review artifacts
-- Drop the current plans infrastructure (from migrations 004 + 013)
DROP INDEX IF EXISTS idx_plans_project;
DROP INDEX IF EXISTS idx_plans_status;
DROP INDEX IF EXISTS idx_plans_issue;
DROP TABLE IF EXISTS plans;

-- New simplified plans table (content lives on filesystem)
CREATE TABLE plans (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Plan review comments (line-level and overall feedback)
CREATE TABLE plan_comments (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    line_number INTEGER,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_plan_comments_plan ON plan_comments(plan_id);
