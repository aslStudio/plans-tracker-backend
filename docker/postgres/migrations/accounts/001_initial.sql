CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text,
    display_name text,
    time_zone text NOT NULL DEFAULT 'UTC',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_not_blank CHECK (email IS NULL OR length(btrim(email)) > 0)
);

CREATE UNIQUE INDEX users_email_lower_unique ON users (lower(email)) WHERE email IS NOT NULL;

CREATE TABLE app_settings (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    use_groups boolean NOT NULL DEFAULT false,
    use_priority boolean NOT NULL DEFAULT false,
    use_habit_categories boolean NOT NULL DEFAULT false,
    task_days_ahead integer NOT NULL DEFAULT 1 CHECK (task_days_ahead >= 0),
    task_recurrence_enabled boolean NOT NULL DEFAULT false,
    task_time_enabled boolean NOT NULL DEFAULT false,
    onboarding_completed boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE calendar_settings (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    show_tasks boolean NOT NULL DEFAULT true,
    show_habits boolean NOT NULL DEFAULT true,
    view_mode text NOT NULL DEFAULT 'day' CHECK (view_mode IN ('day', '3day', 'week', 'month')),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE appearance_settings (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    language text NOT NULL DEFAULT 'ru' CHECK (language IN ('ru', 'en')),
    theme_mode text NOT NULL DEFAULT 'system' CHECK (theme_mode IN ('light', 'dark', 'system')),
    accent text NOT NULL DEFAULT 'turquoise' CHECK (accent IN (
        'turquoise', 'red', 'orange', 'yellow', 'green', 'blue', 'indigo', 'pink', 'softPink'
    )),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notification_settings (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled boolean NOT NULL DEFAULT false,
    tasks_empty_day boolean NOT NULL DEFAULT true,
    tasks_overdue boolean NOT NULL DEFAULT true,
    tasks_end_of_day boolean NOT NULL DEFAULT true,
    habits_half_day boolean NOT NULL DEFAULT true,
    habits_end_of_day boolean NOT NULL DEFAULT true,
    calendar_upcoming boolean NOT NULL DEFAULT true,
    calendar_start boolean NOT NULL DEFAULT true,
    streak_at_risk boolean NOT NULL DEFAULT true,
    streak_broken boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION create_default_settings() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO app_settings (user_id) VALUES (NEW.id);
    INSERT INTO calendar_settings (user_id) VALUES (NEW.id);
    INSERT INTO appearance_settings (user_id) VALUES (NEW.id);
    INSERT INTO notification_settings (user_id) VALUES (NEW.id);
    RETURN NEW;
END;
$$;

CREATE TRIGGER users_create_settings
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION create_default_settings();
