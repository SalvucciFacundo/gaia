-- 0008_project_namespaces.sql — Project namespaces, observation lifecycle, and session metadata.
--
-- Additive only: adds project, workspace_dir, and status columns to observations,
-- and adds goal, next_steps, and relevant_files columns to conversations.
-- user_version gates idempotency.

ALTER TABLE observations ADD COLUMN project TEXT NOT NULL DEFAULT 'global';
ALTER TABLE observations ADD COLUMN workspace_dir TEXT NOT NULL DEFAULT '';
ALTER TABLE observations ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'needs_review'));

CREATE INDEX idx_observations_project_status ON observations(project, status);

ALTER TABLE conversations ADD COLUMN goal TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN next_steps TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN relevant_files TEXT NOT NULL DEFAULT '{}';
