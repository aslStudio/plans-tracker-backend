CREATE INDEX task_occurrence_statuses_user_date_idx
ON task_occurrence_statuses (user_id, occurrence_date);
