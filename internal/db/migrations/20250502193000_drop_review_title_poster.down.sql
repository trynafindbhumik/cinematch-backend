-- Revert: Add title and poster_url back to user_reviews table

ALTER TABLE user_reviews ADD COLUMN title text NOT NULL DEFAULT '';
ALTER TABLE user_reviews ADD COLUMN poster_url text NULL;