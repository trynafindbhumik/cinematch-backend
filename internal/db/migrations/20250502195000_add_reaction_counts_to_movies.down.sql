-- Remove reaction summary columns from movies table
ALTER TABLE movies
DROP COLUMN IF EXISTS total_count,
DROP COLUMN IF EXISTS like_count,
DROP COLUMN IF EXISTS love_count,
DROP COLUMN IF EXISTS dislike_count,
DROP COLUMN IF EXISTS hate_count,
DROP COLUMN IF EXISTS skip_count;

-- Drop indexes
DROP INDEX IF EXISTS idx_movies_total_count;
DROP INDEX IF EXISTS idx_movies_like_count;