-- Drop tables in reverse order of creation
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS user_weekly_suggestion_movies;
DROP TABLE IF EXISTS user_suggestion_movies;
DROP TABLE IF EXISTS user_weekly_suggestions;
DROP TABLE IF EXISTS user_suggestions;
DROP TABLE IF EXISTS user_streaming_services;
DROP TABLE IF EXISTS user_searches;
DROP TABLE IF EXISTS user_reviews;
DROP TABLE IF EXISTS user_movies;
DROP TABLE IF EXISTS user_genres;
DROP TABLE IF EXISTS user_daily_suggestion_movies;
DROP TABLE IF EXISTS user_daily_generation_log;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

-- Drop sequences
DROP SEQUENCE IF EXISTS weekly_suggestions_id_seq;
DROP SEQUENCE IF EXISTS users_id_seq;
DROP SEQUENCE IF EXISTS user_weekly_suggestion_movies_id_seq;
DROP SEQUENCE IF EXISTS user_suggestions_id_seq;
DROP SEQUENCE IF EXISTS user_suggestion_movies_id_seq;
DROP SEQUENCE IF EXISTS user_searches_id_seq;
DROP SEQUENCE IF EXISTS user_reviews_id_seq;
DROP SEQUENCE IF EXISTS user_movies_id_seq;
DROP SEQUENCE IF EXISTS user_daily_suggestion_movies_id_seq;
DROP SEQUENCE IF EXISTS user_daily_generation_log_id_seq;
DROP SEQUENCE IF EXISTS streaming_services_id_seq;
DROP SEQUENCE IF EXISTS genres_id_seq;

-- Drop enum types
DROP TYPE IF EXISTS public.watch_status;
DROP TYPE IF EXISTS public.user_tag;
DROP TYPE IF EXISTS public.suggestion_reaction;