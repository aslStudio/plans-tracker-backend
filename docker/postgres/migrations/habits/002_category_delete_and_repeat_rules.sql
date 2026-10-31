-- A category is optional. Deleting it keeps its habits and clears only category_id;
-- user_id is part of the composite foreign key and must remain intact.
ALTER TABLE habits DROP CONSTRAINT habits_user_id_category_id_fkey;
ALTER TABLE habits ADD CONSTRAINT habits_user_id_category_id_fkey
    FOREIGN KEY (user_id, category_id)
    REFERENCES habit_categories (user_id, id)
    ON DELETE SET NULL (category_id);

-- The mobile repository changes updatedAt whenever a habit is edited. The
-- foreign-key SET NULL is an UPDATE too, so it must refresh this timestamp.
CREATE FUNCTION touch_habit_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$;

CREATE TRIGGER habits_updated_at
    BEFORE UPDATE ON habits
    FOR EACH ROW EXECUTE FUNCTION touch_habit_updated_at();

-- Match the RepeatRule variants used by the mobile app. In particular, an
-- empty weekly rule or a zero-day custom interval would never produce a valid
-- schedule and must not enter the database through a future API client.
CREATE FUNCTION valid_habit_repeat_rule(rule jsonb) RETURNS boolean
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    day_value jsonb;
    day_number integer;
    seen_days integer[] := '{}';
    numeric_value numeric;
BEGIN
    IF jsonb_typeof(rule) IS DISTINCT FROM 'object' THEN
        RETURN false;
    END IF;

    CASE rule->>'type'
        WHEN 'daily' THEN
            RETURN rule = '{"type":"daily"}'::jsonb;
        WHEN 'weekly' THEN
            IF rule - 'type' - 'daysOfWeek' <> '{}'::jsonb THEN
                RETURN false;
            END IF;
            IF jsonb_typeof(rule->'daysOfWeek') IS DISTINCT FROM 'array' THEN
                RETURN false;
            END IF;
            IF jsonb_array_length(rule->'daysOfWeek') = 0 THEN
                RETURN false;
            END IF;
            FOR day_value IN SELECT value FROM jsonb_array_elements(rule->'daysOfWeek') LOOP
                IF jsonb_typeof(day_value) IS DISTINCT FROM 'number' THEN
                    RETURN false;
                END IF;
                numeric_value := (day_value #>> '{}')::numeric;
                IF numeric_value <> trunc(numeric_value)
                    OR numeric_value NOT BETWEEN 0 AND 6 THEN
                    RETURN false;
                END IF;
                day_number := numeric_value::integer;
                IF day_number = ANY(seen_days) THEN
                    RETURN false;
                END IF;
                seen_days := array_append(seen_days, day_number);
            END LOOP;
            RETURN true;
        WHEN 'monthly' THEN
            IF rule - 'type' - 'dayOfMonth' <> '{}'::jsonb THEN
                RETURN false;
            END IF;
            IF jsonb_typeof(rule->'dayOfMonth') IS DISTINCT FROM 'number' THEN
                RETURN false;
            END IF;
            numeric_value := (rule->'dayOfMonth' #>> '{}')::numeric;
            RETURN numeric_value = trunc(numeric_value)
                AND numeric_value BETWEEN 1 AND 31;
        WHEN 'custom' THEN
            IF rule - 'type' - 'everyNDays' <> '{}'::jsonb THEN
                RETURN false;
            END IF;
            IF jsonb_typeof(rule->'everyNDays') IS DISTINCT FROM 'number' THEN
                RETURN false;
            END IF;
            numeric_value := (rule->'everyNDays' #>> '{}')::numeric;
            RETURN numeric_value = trunc(numeric_value)
                AND numeric_value BETWEEN 1 AND 9007199254740991;
        ELSE
            RETURN false;
    END CASE;
END;
$$;

ALTER TABLE habits DROP CONSTRAINT habits_repeat_rule_check;
ALTER TABLE habits ADD CONSTRAINT habits_repeat_rule_check
    CHECK (valid_habit_repeat_rule(repeat_rule));
