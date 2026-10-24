CREATE TABLE habit_categories (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    label text NOT NULL CHECK (length(btrim(label)) > 0),
    color text,
    sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    PRIMARY KEY (user_id, id)
);

CREATE TABLE habits (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    title text NOT NULL CHECK (length(btrim(title)) > 0),
    category_id text,
    note text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    repeat_rule jsonb NOT NULL CHECK (
        jsonb_typeof(repeat_rule) = 'object'
        AND repeat_rule->>'type' IN ('daily', 'weekly', 'monthly', 'custom')
    ),
    start_minute smallint CHECK (start_minute BETWEEN 0 AND 1439),
    duration_minutes smallint NOT NULL DEFAULT 60 CHECK (duration_minutes > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    FOREIGN KEY (user_id, category_id) REFERENCES habit_categories(user_id, id)
);

CREATE INDEX habits_user_category_idx ON habits (user_id, category_id);

CREATE TABLE habit_completions (
    user_id uuid NOT NULL,
    id text NOT NULL DEFAULT gen_random_uuid()::text,
    habit_id text NOT NULL,
    occurrence_date date NOT NULL,
    completed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, habit_id, occurrence_date),
    FOREIGN KEY (user_id, habit_id) REFERENCES habits(user_id, id) ON DELETE CASCADE
);

CREATE INDEX habit_completions_user_date_idx ON habit_completions (user_id, occurrence_date);
