-- Rollback: 000074_kb_share_with_descendants

DO $$ BEGIN RAISE NOTICE '[Migration 000074 DOWN] Dropping share_with_descendants...'; END $$;

ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS share_with_descendants;

DO $$ BEGIN RAISE NOTICE '[Migration 000074 DOWN] Done'; END $$;
