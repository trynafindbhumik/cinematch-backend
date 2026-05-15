-- Add reaction summary columns to movies table
ALTER TABLE movies
ADD COLUMN IF NOT EXISTS total_count int4 NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS like_count int4 NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS love_count int4 NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS dislike_count int4 NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS hate_count int4 NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS skip_count int4 NOT NULL DEFAULT 0;

-- Create index for efficient queries on reaction counts
CREATE INDEX IF NOT EXISTS idx_movies_total_count ON public.movies USING btree (total_count DESC);
CREATE INDEX IF NOT EXISTS idx_movies_like_count ON public.movies USING btree (like_count DESC);