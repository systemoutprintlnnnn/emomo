-- Migration: add vector type metadata for multimodal embedding profiles.

ALTER TABLE meme_vectors
    ADD COLUMN IF NOT EXISTS vector_type TEXT NOT NULL DEFAULT 'image',
    ADD COLUMN IF NOT EXISTS embedding_provider TEXT,
    ADD COLUMN IF NOT EXISTS embedding_mode TEXT NOT NULL DEFAULT 'independent',
    ADD COLUMN IF NOT EXISTS dimension INTEGER,
    ADD COLUMN IF NOT EXISTS input_hash TEXT;

UPDATE meme_vectors
SET vector_type = 'image'
WHERE vector_type IS NULL OR vector_type = '';

UPDATE meme_vectors
SET embedding_mode = 'independent'
WHERE embedding_mode IS NULL OR embedding_mode = '';

DROP INDEX IF EXISTS idx_meme_vectors_md5_collection;

CREATE UNIQUE INDEX IF NOT EXISTS idx_meme_vectors_md5_collection_type
    ON meme_vectors(md5_hash, collection, vector_type);

CREATE INDEX IF NOT EXISTS idx_meme_vectors_vector_type
    ON meme_vectors(vector_type);
