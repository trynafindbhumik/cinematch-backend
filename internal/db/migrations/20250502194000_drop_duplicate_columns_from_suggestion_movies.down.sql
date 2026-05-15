-- Add back duplicate columns to user_suggestion_movies (for rollback purposes)
ALTER TABLE user_suggestion_movies
ADD COLUMN IF NOT EXISTS title text NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS poster_url text NULL,
ADD COLUMN IF NOT EXISTS genres _text NULL,
ADD COLUMN IF NOT EXISTS release_year int2 NULL,
ADD COLUMN IF NOT EXISTS tmdb_rating int2 NULL;

-- Add back duplicate columns to user_weekly_suggestion_movies
ALTER TABLE user_weekly_suggestion_movies
ADD COLUMN IF NOT EXISTS title text NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS poster_url text NULL,
ADD COLUMN IF NOT EXISTS genres _text NULL,
ADD COLUMN IF NOT EXISTS release_year int2 NULL,
ADD COLUMN IF NOT EXISTS tmdb_rating int2 NULL,
ADD COLUMN IF NOT EXISTS "position" int2 NULL;