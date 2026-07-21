-- Read model Progress (AD-8). Không FK chéo schema (AD-10): user_id là ref logic tới identity.users.
CREATE TABLE progress.daily_stats (
    user_id      uuid    NOT NULL,
    day          date    NOT NULL,
    reviews_done integer NOT NULL DEFAULT 0,
    new_done     integer NOT NULL DEFAULT 0,
    retained     integer NOT NULL DEFAULT 0,  -- North Star/ngày: grade>=2 AND scheduled_days>=7
    again        integer NOT NULL DEFAULT 0,
    hard         integer NOT NULL DEFAULT 0,
    good         integer NOT NULL DEFAULT 0,
    easy         integer NOT NULL DEFAULT 0,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, day)
);

CREATE TABLE progress.study_profiles (
    user_id         uuid PRIMARY KEY,
    streak_current  integer NOT NULL DEFAULT 0,
    streak_best     integer NOT NULL DEFAULT 0,
    last_study_date date,                          -- NULL khi chưa có ngày recall thật
    total_reviews   integer NOT NULL DEFAULT 0,
    total_retained  integer NOT NULL DEFAULT 0,    -- tích lũy, KHÔNG reset khi streak reset (FR-32)
    updated_at      timestamptz NOT NULL DEFAULT now()
);
