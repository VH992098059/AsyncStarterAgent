DROP TABLE IF EXISTS agent_run_messages;
ALTER TABLE drafts
    DROP COLUMN IF EXISTS iteration_count,
    DROP COLUMN IF EXISTS quality_score,
    DROP COLUMN IF EXISTS validation_issues;
DROP INDEX IF EXISTS uq_drafts_agent_run_id;
