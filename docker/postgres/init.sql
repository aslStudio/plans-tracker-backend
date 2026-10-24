\set ON_ERROR_STOP on

SELECT 'CREATE DATABASE accounts' WHERE NOT EXISTS (
    SELECT 1 FROM pg_database WHERE datname = 'accounts'
) \gexec
SELECT 'CREATE DATABASE tasks' WHERE NOT EXISTS (
    SELECT 1 FROM pg_database WHERE datname = 'tasks'
) \gexec
SELECT 'CREATE DATABASE habits' WHERE NOT EXISTS (
    SELECT 1 FROM pg_database WHERE datname = 'habits'
) \gexec

\connect accounts
CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '001_initial'
) THEN 'false' ELSE 'true' END AS apply_accounts \gset
\if :apply_accounts
BEGIN;
\i /db-migrations/accounts/001_initial.sql
INSERT INTO schema_migrations (version) VALUES ('001_initial');
COMMIT;
\endif

\connect tasks
CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '001_initial'
) THEN 'false' ELSE 'true' END AS apply_tasks \gset
\if :apply_tasks
BEGIN;
\i /db-migrations/tasks/001_initial.sql
INSERT INTO schema_migrations (version) VALUES ('001_initial');
COMMIT;
\endif

\connect habits
CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '001_initial'
) THEN 'false' ELSE 'true' END AS apply_habits \gset
\if :apply_habits
BEGIN;
\i /db-migrations/habits/001_initial.sql
INSERT INTO schema_migrations (version) VALUES ('001_initial');
COMMIT;
\endif
