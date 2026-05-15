-- Drop user_reaction table if exists
DROP TABLE IF EXISTS user_reaction CASCADE;

-- Rename user_daily_suggestion_movies to user_reaction
ALTER TABLE user_daily_suggestion_movies RENAME TO user_reaction;

-- Remove columns from user_reaction
ALTER TABLE user_reaction DROP COLUMN IF EXISTS description;
ALTER TABLE user_reaction DROP COLUMN IF EXISTS duration;
ALTER TABLE user_reaction DROP COLUMN IF EXISTS language;
ALTER TABLE user_reaction DROP COLUMN IF EXISTS director;

-- Drop generated_count and add movie_ids to user_daily_generation_log
ALTER TABLE user_daily_generation_log DROP COLUMN IF EXISTS generated_count;
ALTER TABLE user_daily_generation_log ADD COLUMN IF NOT EXISTS movie_ids int4[] DEFAULT '{}'::int4[];