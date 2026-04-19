-- Rollback: Re-add columns to memes table and restore data from meme_vectors
-- WARNING: This rollback may lose data if meme_vectors has more records than were migrated

-- Step 1: Add back the columns
ALTER TABLE memes ADD COLUMN IF NOT EXISTS embedding_model TEXT;
ALTER TABLE memes ADD COLUMN IF NOT EXISTS qdrant_point_id TEXT;

-- Step 2: Restore data from meme_vectors (using the 'emomo' collection records)
UPDATE memes m
SET
    embedding_model = mv.embedding_model,
    qdrant_point_id = mv.qdrant_point_id
FROM meme_vectors mv
WHERE m.id = mv.meme_id
  AND mv.collection = 'emomo';

-- Step 3: Optionally delete the migrated records from meme_vectors
-- Commented out for safety - uncomment if you want to remove the migrated records
-- DELETE FROM meme_vectors WHERE collection = 'emomo';
