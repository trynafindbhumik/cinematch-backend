-- Revert user_reaction back to user_daily_suggestion_movies
ALTER TABLE user_reaction RENAME TO user_daily_suggestion_movies;

-- Add back the columns to user_daily_suggestion_movies
ALTER TABLE user_daily_suggestion_movies ADD COLUMN IF NOT EXISTS description text NULL;
ALTER TABLE user_daily_suggestion_movies ADD COLUMN IF NOT EXISTS duration int2 NULL;
ALTER TABLE user_daily_suggestion_movies ADD COLUMN IF NOT EXISTS language varchar(10) NULL;
ALTER TABLE user_daily_suggestion_movies ADD COLUMN IF NOT EXISTS director text NULL;

-- Drop movie_ids and add back generated_count to user_daily_generation_log
ALTER TABLE user_daily_generation_log DROP COLUMN IF EXISTS movie_ids;
ALTER TABLE user_daily_generation_log ADD COLUMN IF NOT EXISTS generated_count int2 DEFAULT 0 NOT NULL;