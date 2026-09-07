-- Roster and business config (data, not hardcoded logic).

CREATE TABLE employees (
    employee_id TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('NV', 'TC', 'PC')),
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE employee_leave_days (
    employee_id TEXT NOT NULL REFERENCES employees(employee_id),
    leave_date  DATE NOT NULL,
    PRIMARY KEY (employee_id, leave_date)
);

CREATE TABLE employee_availability (
    employee_id TEXT NOT NULL REFERENCES employees(employee_id),
    avail_date  DATE NOT NULL,
    sang        BOOLEAN NOT NULL DEFAULT FALSE,
    dem         BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (employee_id, avail_date)
);

CREATE TABLE gates (
    code         TEXT PRIMARY KEY,
    is_lead_gate BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE gate_shift_requirements (
    gate_code           TEXT NOT NULL REFERENCES gates(code),
    shift_type          TEXT NOT NULL CHECK (shift_type IN ('sang', 'dem')),
    nv                  INT NOT NULL DEFAULT 0 CHECK (nv BETWEEN 0 AND 1000),
    lead                INT NOT NULL DEFAULT 0 CHECK (lead BETWEEN 0 AND 1000),
    lead_mandatory_role BOOLEAN NOT NULL DEFAULT FALSE,
    shift_hours         INT NOT NULL CHECK (shift_hours BETWEEN 1 AND 24),
    PRIMARY KEY (gate_code, shift_type)
);

-- Singleton row (id always 1) holding weekly target + objective weights.
CREATE TABLE solver_settings (
    id                     INT PRIMARY KEY CHECK (id = 1),
    target_hours_per_week  INT NOT NULL CHECK (target_hours_per_week BETWEEN 0 AND 168),
    shortfall_penalty      INT NOT NULL CHECK (shortfall_penalty BETWEEN 0 AND 1000000),
    lead_shortfall_penalty INT NOT NULL CHECK (lead_shortfall_penalty BETWEEN 0 AND 1000000),
    balance_penalty_weight INT NOT NULL CHECK (balance_penalty_weight BETWEEN 0 AND 1000000),
    streak_penalty_weight  INT NOT NULL CHECK (streak_penalty_weight BETWEEN 0 AND 1000000),
    streak_length          INT NOT NULL CHECK (streak_length BETWEEN 1 AND 60),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Manager-confirmed state per (employee, date) cell — feeds locked_assignments
-- and carry_in on future solves. Current state only; not a change history.
CREATE TABLE approved_assignments (
    employee_id     TEXT NOT NULL REFERENCES employees(employee_id),
    assignment_date DATE NOT NULL,
    gate            TEXT,
    shift           TEXT CHECK (shift IN ('sang', 'dem')),
    off             BOOLEAN NOT NULL DEFAULT FALSE,
    approved_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    approved_by     TEXT NOT NULL,
    PRIMARY KEY (employee_id, assignment_date),
    CHECK (
        (off AND gate IS NULL AND shift IS NULL) OR
        (NOT off AND gate IS NOT NULL AND shift IS NOT NULL)
    )
);

-- Append-only solve history. One row set per /schedule/solve call; "latest"
-- is just ORDER BY created_at DESC LIMIT 1.
CREATE TABLE schedule_runs (
    id              BIGSERIAL PRIMARY KEY,
    start_date      DATE NOT NULL,
    num_days        INT NOT NULL,
    status          TEXT NOT NULL,
    objective_value DOUBLE PRECISION,
    wall_time_s     DOUBLE PRECISION NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX schedule_runs_created_at_idx ON schedule_runs (created_at DESC);

CREATE TABLE schedule_assignments (
    schedule_run_id BIGINT NOT NULL REFERENCES schedule_runs(id) ON DELETE CASCADE,
    employee_id     TEXT NOT NULL REFERENCES employees(employee_id),
    assignment_date DATE NOT NULL,
    gate            TEXT NOT NULL,
    shift           TEXT NOT NULL CHECK (shift IN ('sang', 'dem')),
    PRIMARY KEY (schedule_run_id, employee_id, assignment_date)
);
CREATE INDEX schedule_assignments_run_date_idx ON schedule_assignments (schedule_run_id, assignment_date);

CREATE TABLE schedule_shortages (
    id              BIGSERIAL PRIMARY KEY,
    schedule_run_id BIGINT NOT NULL REFERENCES schedule_runs(id) ON DELETE CASCADE,
    shortage_date   DATE NOT NULL,
    gate            TEXT NOT NULL,
    shift           TEXT NOT NULL CHECK (shift IN ('sang', 'dem')),
    shortage_type   TEXT NOT NULL CHECK (shortage_type IN ('staff', 'lead')),
    missing         INT NOT NULL
);
CREATE INDEX schedule_shortages_run_idx ON schedule_shortages (schedule_run_id);

CREATE TABLE schedule_employee_summary (
    schedule_run_id BIGINT NOT NULL REFERENCES schedule_runs(id) ON DELETE CASCADE,
    employee_id     TEXT NOT NULL REFERENCES employees(employee_id),
    role            TEXT NOT NULL CHECK (role IN ('NV', 'TC', 'PC')),
    leave_days      INT NOT NULL,
    target_hours    INT NOT NULL,
    actual_hours    INT NOT NULL,
    deviation_hours INT NOT NULL,
    actual_shifts   INT NOT NULL,
    PRIMARY KEY (schedule_run_id, employee_id)
);
