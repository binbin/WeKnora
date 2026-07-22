-- Migration: 000074_kb_share_with_descendants
-- Description: Opt-in flag so a knowledge base owned by an OrgUnit can be
--              read (reference-only) by descendant OrgUnits. Default false.

DO $$ BEGIN RAISE NOTICE '[Migration 000074] Adding share_with_descendants to knowledge_bases...'; END $$;

ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS share_with_descendants BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN knowledge_bases.share_with_descendants IS
    'When true, descendant OrgUnits may read this KB (read-only); default false';

DO $$ BEGIN RAISE NOTICE '[Migration 000074] knowledge_bases.share_with_descendants ready'; END $$;
