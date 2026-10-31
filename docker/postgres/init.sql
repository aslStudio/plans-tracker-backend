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
SELECT 'CREATE DATABASE calendar' WHERE NOT EXISTS (
    SELECT 1 FROM pg_database WHERE datname = 'calendar'
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

SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '002_apple_auth'
) THEN 'false' ELSE 'true' END AS apply_accounts_002 \gset
\if :apply_accounts_002
BEGIN;
\i /db-migrations/accounts/002_apple_auth.sql
INSERT INTO schema_migrations (version) VALUES ('002_apple_auth');
COMMIT;
\endif

SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '003_apple_notifications'
) THEN 'false' ELSE 'true' END AS apply_accounts_003 \gset
\if :apply_accounts_003
BEGIN;
\i /db-migrations/accounts/003_apple_notifications.sql
INSERT INTO schema_migrations (version) VALUES ('003_apple_notifications');
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

SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '002_drop_group_color_check'
) THEN 'false' ELSE 'true' END AS apply_tasks_002 \gset
\if :apply_tasks_002
BEGIN;
\i /db-migrations/tasks/002_drop_group_color_check.sql
INSERT INTO schema_migrations (version) VALUES ('002_drop_group_color_check');
COMMIT;
\endif

SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '003_occurrence_status_date_index'
) THEN 'false' ELSE 'true' END AS apply_tasks_003 \gset
\if :apply_tasks_003
BEGIN;
\i /db-migrations/tasks/003_occurrence_status_date_index.sql
INSERT INTO schema_migrations (version) VALUES ('003_occurrence_status_date_index');
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

SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '002_category_delete_and_repeat_rules'
) THEN 'false' ELSE 'true' END AS apply_habits_002 \gset
\if :apply_habits_002
BEGIN;
\i /db-migrations/habits/002_category_delete_and_repeat_rules.sql
INSERT INTO schema_migrations (version) VALUES ('002_category_delete_and_repeat_rules');
COMMIT;
\endif

\connect calendar
CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
SELECT CASE WHEN EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '001_initial'
) THEN 'false' ELSE 'true' END AS apply_calendar \gset
\if :apply_calendar
BEGIN;
\i /db-migrations/calendar/001_initial.sql
INSERT INTO schema_migrations (version) VALUES ('001_initial');
COMMIT;
\endif
