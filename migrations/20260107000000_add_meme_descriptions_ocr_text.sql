-- Migration: Add OCR text field to meme_descriptions
-- This migration adds ocr_text column for storing extracted image text.

ALTER TABLE meme_descriptions
    ADD COLUMN IF NOT EXISTS ocr_text TEXT;
