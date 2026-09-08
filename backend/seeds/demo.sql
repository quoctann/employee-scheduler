-- Optional development/demo data. This file is deliberately not run by the
-- Kubernetes migration Job. Apply it once with psql to initialize a fresh DB.
INSERT INTO employees (employee_id, name, role) VALUES
    ('NV01', 'Nhan vien 01', 'NV'),
    ('NV02', 'Nhan vien 02', 'NV'),
    ('NV03', 'Nhan vien 03', 'NV'),
    ('NV04', 'Nhan vien 04', 'NV'),
    ('NV05', 'Nhan vien 05', 'NV'),
    ('NV06', 'Nhan vien 06', 'NV'),
    ('NV07', 'Nhan vien 07', 'NV'),
    ('NV08', 'Nhan vien 08', 'NV'),
    ('NV09', 'Nhan vien 09', 'NV'),
    ('NV10', 'Nhan vien 10', 'NV'),
    ('NV11', 'Nhan vien 11', 'NV'),
    ('NV12', 'Nhan vien 12', 'NV'),
    ('NV13', 'Nhan vien 13', 'NV'),
    ('NV14', 'Nhan vien 14', 'NV'),
    ('NV15', 'Nhan vien 15', 'NV'),
    ('PC16', 'Pho ca 16', 'PC'),
    ('PC17', 'Pho ca 17', 'PC'),
    ('PC18', 'Pho ca 18', 'PC'),
    ('TC19', 'Truong ca 19', 'TC'),
    ('TC20', 'Truong ca 20', 'TC'),
    ('TC21', 'Truong ca 21', 'TC');

INSERT INTO gates (code, is_lead_gate) VALUES
    ('A', FALSE), ('B', TRUE), ('G', FALSE), ('D', FALSE);

INSERT INTO gate_shift_requirements (gate_code, shift_type, nv, lead, lead_mandatory_role, shift_hours) VALUES
    ('A', 'sang', 1, 0, FALSE, 11), ('A', 'dem', 1, 0, FALSE, 13),
    ('B', 'sang', 2, 1, FALSE, 11), ('B', 'dem', 1, 1, TRUE, 12),
    ('G', 'sang', 1, 0, FALSE, 11), ('G', 'dem', 1, 0, FALSE, 13),
    ('D', 'sang', 1, 0, FALSE, 11), ('D', 'dem', 2, 0, FALSE, 12);

INSERT INTO solver_settings
    (id, target_hours_per_week, shortfall_penalty, lead_shortfall_penalty, balance_penalty_weight, streak_penalty_weight, streak_length)
VALUES (1, 44, 1000, 800, 1, 2, 3);

INSERT INTO employee_availability (employee_id, avail_date, sang, dem)
SELECT e.employee_id, d::date, TRUE, TRUE
FROM employees e
CROSS JOIN generate_series(CURRENT_DATE, CURRENT_DATE + INTERVAL '27 days', INTERVAL '1 day') AS d;

UPDATE employee_availability SET sang = FALSE, dem = FALSE
    WHERE employee_id = 'NV05' AND avail_date = CURRENT_DATE + 3;
UPDATE employee_availability SET sang = FALSE, dem = FALSE
    WHERE employee_id = 'NV12' AND avail_date = CURRENT_DATE + 10;
UPDATE employee_availability SET sang = FALSE, dem = FALSE
    WHERE employee_id = 'PC17' AND avail_date = CURRENT_DATE + 14;

INSERT INTO employee_leave_days (employee_id, leave_date)
SELECT 'NV08', CURRENT_DATE + n FROM generate_series(5, 9) AS n;
UPDATE employee_availability SET sang = FALSE, dem = FALSE
    WHERE employee_id = 'NV08' AND avail_date BETWEEN CURRENT_DATE + 5 AND CURRENT_DATE + 9;
