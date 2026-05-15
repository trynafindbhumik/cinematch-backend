-- Drop title and poster_url from user_reviews table
-- These fields should be fetched from movies table on read

ALTER TABLE user_reviews DROP COLUMN IF EXISTS title;
ALTER TABLE user_reviews DROP COLUMN IF EXISTS poster_url;