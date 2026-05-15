-- Down migration for 20250501100000_init_schema
-- Drops all tables and schema

DROP SCHEMA public CASCADE;
CREATE SCHEMA public AUTHORIZATION postgres;