CREATE TABLE calendar_events (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    title text NOT NULL CHECK (length(btrim(title)) > 0),
    event_date date NOT NULL,
    start_minute smallint NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    duration_minutes smallint NOT NULL CHECK (duration_minutes > 0 AND start_minute + duration_minutes <= 1440),
    all_day boolean NOT NULL DEFAULT false,
    color text NOT NULL CHECK (length(btrim(color)) > 0),
    description text,
    notes text NOT NULL DEFAULT '',
    location text NOT NULL DEFAULT '',
    repeat_rule jsonb,
    repeat_until date CHECK (repeat_until IS NULL OR repeat_until >= event_date),
    excluded_dates date[] NOT NULL DEFAULT '{}',
    linked_type text CHECK (linked_type IN ('task', 'habit')),
    linked_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    CHECK ((linked_type IS NULL) = (linked_id IS NULL))
);
CREATE INDEX calendar_events_user_date_idx ON calendar_events (user_id, event_date);

CREATE TABLE calendar_settings (
    user_id uuid PRIMARY KEY,
    show_tasks boolean NOT NULL DEFAULT true,
    show_habits boolean NOT NULL DEFAULT true,
    view_mode text NOT NULL DEFAULT 'day' CHECK (view_mode IN ('day', '3day', 'week', 'month')),
    updated_at timestamptz NOT NULL DEFAULT now()
);
