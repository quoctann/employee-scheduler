-- Scoped to exactly what this migration's up.sql inserted, not a blanket
-- truncate — safe even if the shared dev database has picked up other rows
-- since (e.g. from manual testing) in these same tables.
DELETE FROM employee_leave_days WHERE employee_id LIKE 'NV%' OR employee_id LIKE 'PC%' OR employee_id LIKE 'TC%';
DELETE FROM employee_availability WHERE employee_id LIKE 'NV%' OR employee_id LIKE 'PC%' OR employee_id LIKE 'TC%';
DELETE FROM gate_shift_requirements WHERE gate_code IN ('A', 'B', 'G', 'D');
DELETE FROM gates WHERE code IN ('A', 'B', 'G', 'D');
DELETE FROM solver_settings WHERE id = 1;
DELETE FROM employees WHERE employee_id LIKE 'NV%' OR employee_id LIKE 'PC%' OR employee_id LIKE 'TC%';
