\set ON_ERROR_STOP on

\connect accounts
INSERT INTO users (id, email, display_name, time_zone)
VALUES (
    '8dc53fe1-0f08-4bd0-a3ed-6a5541859490',
    'dev@example.invalid',
    'Dev User',
    'Europe/Istanbul'
)
ON CONFLICT (id) DO NOTHING;

\connect tasks
SET TIME ZONE 'Europe/Istanbul';
SELECT ensure_task_defaults('8dc53fe1-0f08-4bd0-a3ed-6a5541859490'::uuid);

INSERT INTO tasks (
    user_id, id, title, note, status_id, priority_id,
    start_date, end_date, start_minute, duration_minutes, repeat_rule
)
VALUES
    (
        '8dc53fe1-0f08-4bd0-a3ed-6a5541859490',
        '7fbcc6a7-ac8a-43b1-8ff8-1d65730a2be1',
        'Разобраться с pgx',
        'Прочитать, как работают Query, Scan и rows.Err',
        'status-todo',
        'priority-high',
        CURRENT_DATE,
        CURRENT_DATE,
        600,
        60,
        NULL
    ),
    (
        '8dc53fe1-0f08-4bd0-a3ed-6a5541859490',
        '53e8cbf0-60b1-47f4-ab85-c12aaf707359',
        'Написать GET /v1/tasks',
        '',
        'status-todo',
        'priority-medium',
        CURRENT_DATE + 1,
        CURRENT_DATE + 1,
        NULL,
        60,
        NULL
    ),
    (
        '8dc53fe1-0f08-4bd0-a3ed-6a5541859490',
        'c6e2d14d-8d79-4fa1-9b1f-648425979743',
        'Подготовить схему задач',
        '',
        'status-done',
        NULL,
        CURRENT_DATE - 1,
        CURRENT_DATE - 1,
        NULL,
        60,
        NULL
    ),
    (
        '8dc53fe1-0f08-4bd0-a3ed-6a5541859490',
        '1ae18684-3dd8-4bd3-ab5f-1184d10d6e9a',
        'Задача без даты',
        '',
        'status-todo',
        NULL,
        NULL,
        NULL,
        NULL,
        60,
        NULL
    )
ON CONFLICT (user_id, id) DO NOTHING;
