CREATE TABLE task_statuses (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    label text NOT NULL CHECK (length(btrim(label)) > 0),
    color text,
    sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    is_completed boolean NOT NULL DEFAULT false,
    is_built_in boolean NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, id)
);

CREATE TABLE task_priorities (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    label text NOT NULL CHECK (length(btrim(label)) > 0),
    color text,
    sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    is_built_in boolean NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, id)
);

CREATE TABLE task_groups (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    label text NOT NULL CHECK (length(btrim(label)) > 0),
    color text NOT NULL CHECK (color IN (
        'accent-turquoise', 'accent-red', 'accent-orange', 'accent-yellow',
        'accent-green', 'accent-blue', 'accent-indigo', 'accent-pink'
    )),
    sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    PRIMARY KEY (user_id, id)
);

CREATE TABLE tasks (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    title text NOT NULL CHECK (length(btrim(title)) > 0),
    note text NOT NULL DEFAULT '',
    description text,
    status_id text NOT NULL DEFAULT 'status-todo',
    priority_id text,
    group_id text,
    start_date date,
    end_date date,
    start_minute smallint CHECK (start_minute BETWEEN 0 AND 1439),
    duration_minutes smallint NOT NULL DEFAULT 60 CHECK (duration_minutes > 0),
    repeat_rule jsonb CHECK (
        repeat_rule IS NULL OR (
            jsonb_typeof(repeat_rule) = 'object'
            AND repeat_rule->>'type' IN ('daily', 'weekly', 'monthly', 'custom')
        )
    ),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    CHECK ((start_date IS NULL AND end_date IS NULL) OR
           (start_date IS NOT NULL AND end_date IS NOT NULL AND end_date >= start_date)),
    FOREIGN KEY (user_id, status_id) REFERENCES task_statuses(user_id, id),
    FOREIGN KEY (user_id, priority_id) REFERENCES task_priorities(user_id, id),
    FOREIGN KEY (user_id, group_id) REFERENCES task_groups(user_id, id)
);

CREATE INDEX tasks_user_dates_idx ON tasks (user_id, start_date, end_date);
CREATE INDEX tasks_user_status_idx ON tasks (user_id, status_id);
CREATE INDEX tasks_user_group_idx ON tasks (user_id, group_id);

CREATE TABLE task_occurrence_statuses (
    user_id uuid NOT NULL,
    task_id text NOT NULL,
    occurrence_date date NOT NULL,
    status_id text NOT NULL,
    PRIMARY KEY (user_id, task_id, occurrence_date),
    FOREIGN KEY (user_id, task_id) REFERENCES tasks(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, status_id) REFERENCES task_statuses(user_id, id)
);

CREATE TABLE task_completions (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    task_id text NOT NULL,
    occurrence_date date NOT NULL,
    completed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, task_id, occurrence_date),
    FOREIGN KEY (user_id, task_id) REFERENCES tasks(user_id, id) ON DELETE CASCADE
);

CREATE INDEX task_completions_user_date_idx ON task_completions (user_id, occurrence_date);

CREATE TABLE calendar_events (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    title text NOT NULL CHECK (length(btrim(title)) > 0),
    event_date date NOT NULL,
    start_minute smallint NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    duration_minutes smallint NOT NULL CHECK (duration_minutes > 0),
    all_day boolean NOT NULL DEFAULT false,
    color text NOT NULL,
    description text,
    notes text NOT NULL DEFAULT '',
    location text NOT NULL DEFAULT '',
    repeat_rule jsonb CHECK (
        repeat_rule IS NULL OR (
            jsonb_typeof(repeat_rule) = 'object'
            AND repeat_rule->>'type' IN ('daily', 'weekly', 'monthly', 'custom')
        )
    ),
    repeat_until date,
    excluded_dates date[] NOT NULL DEFAULT '{}',
    linked_type text CHECK (linked_type IN ('task', 'habit')),
    linked_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    CHECK (start_minute + duration_minutes <= 1440),
    CHECK (repeat_until IS NULL OR repeat_until >= event_date),
    CHECK ((linked_type IS NULL) = (linked_id IS NULL))
);

CREATE INDEX calendar_events_user_date_idx ON calendar_events (user_id, event_date);
CREATE INDEX calendar_events_user_link_idx ON calendar_events (user_id, linked_type, linked_id)
    WHERE linked_type IS NOT NULL;

-- Built-in IDs match the current mobile repository. Call this for each new user.
CREATE FUNCTION ensure_task_defaults(p_user_id uuid) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO task_statuses (user_id, id, label, sort_order, is_completed, is_built_in)
    VALUES (p_user_id, 'status-todo', 'К выполнению', 0, false, true)
    ON CONFLICT (user_id, id) DO NOTHING;

    INSERT INTO task_statuses (user_id, id, label, color, sort_order, is_completed, is_built_in)
    VALUES (p_user_id, 'status-done', 'Готово', 'accent-green', 1, true, true)
    ON CONFLICT (user_id, id) DO NOTHING;

    INSERT INTO task_priorities (user_id, id, label, color, sort_order, is_built_in)
    VALUES
        (p_user_id, 'priority-low', 'Низкий', 'gray-gray', 0, true),
        (p_user_id, 'priority-medium', 'Средний', 'accent-yellow', 1, true),
        (p_user_id, 'priority-high', 'Высокий', 'accent-red', 2, true)
    ON CONFLICT (user_id, id) DO NOTHING;
END;
$$;

CREATE FUNCTION seed_task_defaults_before_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM ensure_task_defaults(NEW.user_id);
    RETURN NEW;
END;
$$;

CREATE TRIGGER tasks_seed_defaults
BEFORE INSERT ON tasks
FOR EACH ROW EXECUTE FUNCTION seed_task_defaults_before_insert();
