-- Migration: Add meme_descriptions table for multi-VLM model support
-- This migration:
-- 1. Creates meme_descriptions table
-- 2. Adds description_id column to meme_vectors
-- 3. Migrates existing vlm_description data from memes table
-- 4. Links existing meme_vectors to their descriptions
-- 5. Removes vlm_description and vlm_model from memes table

-- Step 1: Create meme_descriptions table
CREATE TABLE IF NOT EXISTS meme_descriptions (
    id TEXT PRIMARY KEY,
    meme_id TEXT NOT NULL,
    md5_hash TEXT NOT NULL,
    vlm_model TEXT NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Create indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_meme_descriptions_md5_model
    ON meme_descriptions(md5_hash, vlm_model);
CREATE INDEX IF NOT EXISTS idx_meme_descriptions_meme
    ON meme_descriptions(meme_id);

-- Step 3: Migrate existing VLM descriptions from memes table
-- Use DISTINCT ON (PostgreSQL) to handle duplicate md5_hash
INSERT INTO meme_descriptions (id, meme_id, md5_hash, vlm_model, description, created_at)
SELECT
    gen_random_uuid()::text,
    m.id,
    m.md5_hash,
    COALESCE(NULLIF(m.vlm_model, ''), 'gpt-4o-mini'),
    m.vlm_description,
    m.created_at
FROM (
    SELECT DISTINCT ON (md5_hash) *
    FROM memes
    WHERE vlm_description IS NOT NULL
      AND vlm_description != ''
    ORDER BY md5_hash, created_at DESC
) m
WHERE NOT EXISTS (
    SELECT 1 FROM meme_descriptions md
    WHERE md.md5_hash = m.md5_hash
      AND md.vlm_model = COALESCE(NULLIF(m.vlm_model, ''), 'gpt-4o-mini')
);

-- Step 4: Add description_id column to meme_vectors
ALTER TABLE meme_vectors ADD COLUMN IF NOT EXISTS description_id TEXT;

-- Step 5: Link existing meme_vectors to their descriptions
-- Match by md5_hash and the legacy vlm_model from the associated meme
UPDATE meme_vectors mv
SET description_id = md.id
FROM meme_descriptions md
JOIN memes m ON md.meme_id = m.id
WHERE mv.md5_hash = md.md5_hash
  AND mv.description_id IS NULL
  AND md.vlm_model = COALESCE(NULLIF(m.vlm_model, ''), 'gpt-4o-mini');

-- Step 6: Create index on description_id for efficient joins
CREATE INDEX IF NOT EXISTS idx_meme_vectors_description
    ON meme_vectors(description_id);

-- Step 7: Remove vlm_description and vlm_model from memes table
ALTER TABLE memes DROP COLUMN IF EXISTS vlm_description;
ALTER TABLE memes DROP COLUMN IF EXISTS vlm_model;
