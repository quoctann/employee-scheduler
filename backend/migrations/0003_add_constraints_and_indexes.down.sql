ALTER TABLE schedule_runs DROP CONSTRAINT IF EXISTS schedule_runs_status_check;
ALTER TABLE gate_shift_requirements DROP CONSTRAINT IF EXISTS gate_shift_requirements_lead_mandatory_check;

DROP INDEX IF EXISTS approved_assignments_assignment_date_idx;
DROP INDEX IF EXISTS employee_availability_avail_date_idx;

ALTER TABLE schedule_shortages DROP CONSTRAINT IF EXISTS schedule_shortages_gate_fkey;
ALTER TABLE schedule_assignments DROP CONSTRAINT IF EXISTS schedule_assignments_gate_fkey;
ALTER TABLE approved_assignments DROP CONSTRAINT IF EXISTS approved_assignments_gate_fkey;
