-- Drop duplicate columns from user_suggestion_movies (title, poster_url, genres, release_year, tmdb_rating already exist in movies table)
ALTER TABLE user_suggestion_movies
DROP COLUMN IF EXISTS title,
DROP COLUMN IF EXISTS poster_url,
DROP COLUMN IF EXISTS genres,
DROP COLUMN IF EXISTS release_year,
DROP COLUMN IF EXISTS tmdb_rating;

-- Drop duplicate columns from user_weekly_suggestion_movies
ALTER TABLE user_weekly_suggestion_movies
DROP COLUMN IF EXISTS title,
DROP COLUMN IF EXISTS poster_url,
DROP COLUMN IF EXISTS genres,
DROP COLUMN IF EXISTS release_year,
DROP COLUMN IF EXISTS tmdb_rating;

-- Drop duplicate columns from user_daily_suggestion_movies (keep description, duration, language, director as they are suggestion-specific)
ALTER TABLE user_daily_suggestion_movies
DROP COLUMN IF EXISTS title,
DROP COLUMN IF EXISTS poster_url,
DROP COLUMN IF EXISTS genres,
DROP COLUMN IF EXISTS release_year,
DROP COLUMN IF EXISTS tmdb_rating;