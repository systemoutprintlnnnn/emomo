-- Migration: Move embedding_model and qdrant_point_id from memes to meme_vectors
-- Then drop the columns from memes table

-- Step 1: Insert missing records into meme_vectors from memes
-- Only insert records that don't already exist in meme_vectors (by md5_hash + collection)
-- Use DISTINCT ON to handle duplicate md5_hash in memes table
INSERT INTO meme_vectors (id, meme_id, md5_hash, collection, embedding_model, qdrant_point_id, status, created_at)
SELECT
    gen_random_uuid()::text,
    m.id,
    m.md5_hash,
    'emomo',  -- Default collection for legacy data
    COALESCE(m.embedding_model, 'jina-embeddings-v3'),  -- Default to jina if null
    m.qdrant_point_id,
    'active',
    m.created_at
FROM (
    SELECT DISTINCT ON (md5_hash) *
    FROM memes
    WHERE qdrant_point_id IS NOT NULL
      AND qdrant_point_id != ''
    ORDER BY md5_hash, created_at DESC
) m
WHERE NOT EXISTS (
    SELECT 1 FROM meme_vectors mv
    WHERE mv.md5_hash = m.md5_hash
      AND mv.collection = 'emomo'
);

-- Step 2: Drop the columns from memes table
ALTER TABLE memes DROP COLUMN IF EXISTS embedding_model;
ALTER TABLE memes DROP COLUMN IF EXISTS qdrant_point_id;
