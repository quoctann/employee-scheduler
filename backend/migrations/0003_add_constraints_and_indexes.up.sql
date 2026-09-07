-- Follow-up from database review: `gate` columns had no referential
-- integrity check, and date-range queries had no supporting index since the
-- leading PK column (employee_id / schedule_run_id) isn't part of their
-- WHERE clause.

ALTER TABLE approved_assignments
    ADD CONSTRAINT approved_assignments_gate_fkey FOREIGN KEY (gate) REFERENCES gates(code);
ALTER TABLE schedule_assignments
    ADD CONSTRAINT schedule_assignments_gate_fkey FOREIGN KEY (gate) REFERENCES gates(code);
ALTER TABLE schedule_shortages
    ADD CONSTRAINT schedule_shortages_gate_fkey FOREIGN KEY (gate) REFERENCES gates(code);

CREATE INDEX employee_availability_avail_date_idx ON employee_availability (avail_date);
CREATE INDEX approved_assignments_assignment_date_idx ON approved_assignments (assignment_date);

-- A slot can only be a mandatory-lead slot if it actually asks for a lead.
ALTER TABLE gate_shift_requirements
    ADD CONSTRAINT gate_shift_requirements_lead_mandatory_check CHECK (NOT lead_mandatory_role OR lead > 0);

-- Matches domain.SolveStatus exactly, same enum-CHECK pattern as role/shift elsewhere.
ALTER TABLE schedule_runs
    ADD CONSTRAINT schedule_runs_status_check
    CHECK (status IN ('OPTIMAL', 'FEASIBLE', 'INFEASIBLE', 'UNKNOWN', 'MODEL_INVALID'));
