-- CineMatch Backend Initial Schema
-- Version: 20250501100000

-- Drop existing schema if exists (only run this in development)
DROP SCHEMA public CASCADE;
CREATE SCHEMA public AUTHORIZATION postgres;

-- Create ENUM types
CREATE TYPE public.suggestion_reaction AS ENUM (
	'hate',
	'dislike',
	'skip',
	'like',
	'love');

CREATE TYPE public.user_tag AS ENUM (
	'screen_enthusiast',
	'cinema_lover',
	'cinephile',
	'cinephile_pro',
	'cinephile_elite');

CREATE TYPE public.watch_status AS ENUM (
	'watchlist',
	'watched',
	'none');

CREATE TYPE public.verification_type AS ENUM (
	'signup',
	'password_reset',
	'email_change_old',
	'email_change_new');

-- Create sequences
CREATE SEQUENCE genres_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 32767 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE streaming_services_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 32767 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE user_daily_generation_log_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE user_daily_suggestion_movies_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE user_movies_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE user_reviews_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE user_searches_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE user_suggestion_movies_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE user_suggestions_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE user_weekly_suggestion_movies_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE users_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE weekly_suggestions_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;
CREATE SEQUENCE email_verifications_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE;

-- Create genres table
CREATE TABLE genres (
	id smallserial NOT NULL,
	"name" text NOT NULL,
	tmdb_id int4 NULL,
	CONSTRAINT genres_name_key UNIQUE (name),
	CONSTRAINT genres_pkey PRIMARY KEY (id),
	CONSTRAINT genres_tmdb_id_key UNIQUE (tmdb_id)
);

-- Create streaming_services table
CREATE TABLE streaming_services (
	id smallserial NOT NULL,
	"name" text NOT NULL,
	icon_url text NULL,
	tmdb_id int4 NULL,
	CONSTRAINT streaming_services_name_key UNIQUE (name),
	CONSTRAINT streaming_services_pkey PRIMARY KEY (id)
);
CREATE UNIQUE INDEX idx_streaming_services_tmdb_id ON public.streaming_services USING btree (tmdb_id) WHERE (tmdb_id IS NOT NULL);

-- Create movies table
CREATE TABLE movies (
	id serial4 NOT NULL,
	tmdb_id int4 NOT NULL,
	title text NOT NULL,
	poster_url text NULL,
	genres _text NULL,
	release_year int2 NULL,
	tmdb_rating int2 NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	updated_at timestamptz DEFAULT now() NOT NULL,
	backdrop_url text NULL,
	CONSTRAINT movies_pkey PRIMARY KEY (id),
	CONSTRAINT movies_tmdb_id_key UNIQUE (tmdb_id)
);
CREATE INDEX idx_movies_title ON public.movies USING btree (title);
CREATE INDEX idx_movies_tmdb_id ON public.movies USING btree (tmdb_id);

-- Create tag_suggestion_config table
CREATE TABLE tag_suggestion_config (
	tag public.user_tag NOT NULL,
	suggestions_per_week int2 NOT NULL,
	movies_per_suggestion int2 NOT NULL,
	CONSTRAINT tag_suggestion_config_pkey PRIMARY KEY (tag)
);

-- Create users table
-- Note: disabled_at removed (using is_disabled + disabled_until instead)
CREATE TABLE users (
	id int8 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1 NO CYCLE) NOT NULL,
	public_id uuid DEFAULT gen_random_uuid() NOT NULL,
	email text NOT NULL,
	password_hash text NOT NULL,
	"role" text DEFAULT 'user'::text NOT NULL,
	tag public.user_tag DEFAULT 'screen_enthusiast'::user_tag NOT NULL,
	is_first_login bool DEFAULT true NOT NULL,
	is_deleted bool DEFAULT false NOT NULL,
	deleted_at timestamptz NULL,
	deletion_scheduled_at timestamptz NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	updated_at timestamptz DEFAULT now() NOT NULL,
	smart_suggest bool DEFAULT false NOT NULL,
	failed_attempts int4 DEFAULT 0 NULL,
	lockout_until timestamptz NULL,
	is_verified bool DEFAULT false NOT NULL,
	"name" text DEFAULT ''::text NOT NULL,
	profile_url text NULL,
	is_disabled bool DEFAULT false NULL,
	disabled_until timestamptz NULL,
	previously_disabled_at timestamp NULL,
	CONSTRAINT users_email_key UNIQUE (email),
	CONSTRAINT users_pkey PRIMARY KEY (id),
	CONSTRAINT users_public_id_key UNIQUE (public_id)
);
CREATE INDEX idx_users_disabled_until ON public.users USING btree (disabled_until) WHERE (disabled_until IS NOT NULL);
CREATE INDEX idx_users_is_deleted_deleted_at ON public.users USING btree (is_deleted, deleted_at) WHERE (is_deleted = true);

-- Create sessions table
-- Note: jti_hash added for storing hashed JWT ID (jti)
CREATE TABLE sessions (
	id uuid DEFAULT gen_random_uuid() NOT NULL,
	user_id int8 NOT NULL,
	refresh_token_hash text NOT NULL,
	jti_hash text NULL,
	device_name text NULL,
	user_agent text NULL,
	ip_address inet NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	last_used_at timestamptz DEFAULT now() NOT NULL,
	expires_at timestamptz NULL,
	CONSTRAINT sessions_pkey PRIMARY KEY (id),
	CONSTRAINT sessions_refresh_token_hash_key UNIQUE (refresh_token_hash),
	CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_sessions_user_expires ON public.sessions USING btree (user_id, expires_at);
CREATE INDEX idx_sessions_jti_hash ON public.sessions USING btree (jti_hash) WHERE (jti_hash IS NOT NULL);

-- email_verifications table removed - using Redis instead for OTP storage
-- Keeping the sequence for now in case we need to reference it, but table is gone

-- Create user_daily_generation_log table
CREATE TABLE user_daily_generation_log (
	id bigserial NOT NULL,
	user_id int8 NOT NULL,
	"date" date NOT NULL,
	generated_count int2 DEFAULT 0 NOT NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT user_daily_generation_log_pkey PRIMARY KEY (id),
	CONSTRAINT user_daily_generation_log_user_id_date_key UNIQUE (user_id, date),
	CONSTRAINT user_daily_generation_log_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_generation_user_date ON public.user_daily_generation_log USING btree (user_id, date);

-- Create user_daily_suggestion_movies table
CREATE TABLE user_daily_suggestion_movies (
	id bigserial NOT NULL,
	user_id int8 NOT NULL,
	movie_id int4 NOT NULL,
	title text NOT NULL,
	poster_url text NULL,
	description text NULL,
	genres text NULL,
	release_year int2 NULL,
	duration int2 NULL,
	"language" varchar(10) NULL,
	director text NULL,
	tmdb_rating int2 NULL,
	reaction public.suggestion_reaction NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT user_daily_suggestion_movies_pkey PRIMARY KEY (id),
	CONSTRAINT uniq_daily_user_movie UNIQUE (user_id, movie_id),
	CONSTRAINT user_daily_suggestion_movies_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_daily_queue ON public.user_daily_suggestion_movies USING btree (user_id, reaction) WHERE (reaction IS NULL);

-- Create user_genres table
CREATE TABLE user_genres (
	user_id int8 NOT NULL,
	genre_id int2 NOT NULL,
	CONSTRAINT user_genres_pkey PRIMARY KEY (user_id, genre_id),
	CONSTRAINT user_genres_genre_id_fkey FOREIGN KEY (genre_id) REFERENCES genres(id) ON DELETE CASCADE,
	CONSTRAINT user_genres_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create user_movies table
CREATE TABLE user_movies (
	id bigserial NOT NULL,
	user_id int8 NOT NULL,
	movie_id int4 NOT NULL,
	status public.watch_status DEFAULT 'none'::watch_status NOT NULL,
	is_favorite bool DEFAULT false NULL,
	added_at timestamptz DEFAULT now() NOT NULL,
	updated_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT user_movies_pkey PRIMARY KEY (id),
	CONSTRAINT user_movies_user_id_movie_id_key UNIQUE (user_id, movie_id),
	CONSTRAINT user_movies_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES movies(id) ON DELETE CASCADE,
	CONSTRAINT user_movies_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_movies_favorite ON public.user_movies USING btree (user_id, is_favorite);
CREATE INDEX idx_user_movies_movie_id ON public.user_movies USING btree (movie_id);
CREATE INDEX idx_user_movies_status ON public.user_movies USING btree (user_id, status);
CREATE INDEX idx_user_movies_user_id ON public.user_movies USING btree (user_id);

-- Create user_reviews table
CREATE TABLE user_reviews (
	id bigserial NOT NULL,
	user_id int8 NOT NULL,
	movie_id int4 NOT NULL,
	title text NOT NULL,
	poster_url text NULL,
	rating int2 NULL,
	"comment" text NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT user_reviews_pkey PRIMARY KEY (id),
	CONSTRAINT user_reviews_user_id_movie_id_key UNIQUE (user_id, movie_id),
	CONSTRAINT user_reviews_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create user_searches table
CREATE TABLE user_searches (
	id bigserial NOT NULL,
	user_id int8 NOT NULL,
	query text NOT NULL,
	searched_at timestamptz DEFAULT now() NOT NULL,
	"source" text NULL,
	CONSTRAINT user_searches_pkey PRIMARY KEY (id),
	CONSTRAINT user_searches_user_id_query_key UNIQUE (user_id, query),
	CONSTRAINT user_searches_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_searches_user_time ON public.user_searches USING btree (user_id, searched_at DESC);

-- Create user_streaming_services table
CREATE TABLE user_streaming_services (
	user_id int8 NOT NULL,
	service_id int2 NOT NULL,
	CONSTRAINT user_streaming_services_pkey PRIMARY KEY (user_id, service_id),
	CONSTRAINT user_streaming_services_service_id_fkey FOREIGN KEY (service_id) REFERENCES streaming_services(id) ON DELETE CASCADE,
	CONSTRAINT user_streaming_services_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create user_suggestions table
CREATE TABLE user_suggestions (
	id bigserial NOT NULL,
	user_id int8 NOT NULL,
	week_start date NOT NULL,
	suggestion_index int2 NOT NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT user_suggestions_pkey PRIMARY KEY (id),
	CONSTRAINT user_suggestions_user_id_week_start_suggestion_index_key UNIQUE (user_id, week_start, suggestion_index),
	CONSTRAINT user_suggestions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_suggestions_created ON public.user_suggestions USING btree (user_id, created_at DESC);

-- Create user_weekly_suggestions table
CREATE TABLE user_weekly_suggestions (
	id int8 DEFAULT nextval('weekly_suggestions_id_seq'::regclass) NOT NULL,
	user_id int8 NOT NULL,
	week_start date NOT NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT weekly_suggestions_pkey PRIMARY KEY (id),
	CONSTRAINT weekly_suggestions_user_id_week_start_key UNIQUE (user_id, week_start),
	CONSTRAINT weekly_suggestions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_weekly_user_created ON public.user_weekly_suggestions USING btree (user_id, created_at DESC);

-- Create user_suggestion_movies table
CREATE TABLE user_suggestion_movies (
	id bigserial NOT NULL,
	suggestion_id int8 NOT NULL,
	movie_id int4 NOT NULL,
	title text NOT NULL,
	poster_url text NULL,
	genres _text NULL,
	release_year int2 NULL,
	tmdb_rating int2 NULL,
	"position" int2 NOT NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT user_suggestion_movies_pkey PRIMARY KEY (id),
	CONSTRAINT user_suggestion_movies_suggestion_id_fkey FOREIGN KEY (suggestion_id) REFERENCES user_suggestions(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_suggestion_movies_position ON public.user_suggestion_movies USING btree (suggestion_id, "position");

-- Create user_weekly_suggestion_movies table
CREATE TABLE user_weekly_suggestion_movies (
	id bigserial NOT NULL,
	suggestion_id int8 NOT NULL,
	movie_id int4 NOT NULL,
	title text NOT NULL,
	poster_url text NULL,
	genres _text NULL,
	release_year int2 NULL,
	tmdb_rating int2 NULL,
	"position" int2 NULL,
	created_at timestamptz DEFAULT now() NOT NULL,
	CONSTRAINT user_weekly_suggestion_movies_pkey PRIMARY KEY (id),
	CONSTRAINT user_weekly_suggestion_movies_suggestion_id_movie_id_key UNIQUE (suggestion_id, movie_id),
	CONSTRAINT user_weekly_suggestion_movies_suggestion_id_fkey FOREIGN KEY (suggestion_id) REFERENCES user_weekly_suggestions(id) ON DELETE CASCADE
);
CREATE INDEX idx_weekly_suggestion_movies_position ON public.user_weekly_suggestion_movies USING btree (suggestion_id, "position");

-- Create schema_migrations table for migrate library tracking
CREATE TABLE schema_migrations (
	"version" int8 NOT NULL,
	dirty bool NOT NULL,
	CONSTRAINT schema_migrations_pkey PRIMARY KEY (version)
);

-- Insert initial migration version (20250501100000)
INSERT INTO schema_migrations ("version", dirty) VALUES (20250501100000, false);