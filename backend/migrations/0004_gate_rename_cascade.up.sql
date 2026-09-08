-- Renaming a gate (`gates.code`) must cascade to every table that stores
-- that code, instead of failing with a FK violation the moment any
-- historical row references it. All four FKs onto gates(code) get
-- ON UPDATE CASCADE so a single `UPDATE gates SET code = ...` propagates.

ALTER TABLE gate_shift_requirements DROP CONSTRAINT gate_shift_requirements_gate_code_fkey;
ALTER TABLE gate_shift_requirements
    ADD CONSTRAINT gate_shift_requirements_gate_code_fkey
    FOREIGN KEY (gate_code) REFERENCES gates(code) ON UPDATE CASCADE;

ALTER TABLE approved_assignments DROP CONSTRAINT approved_assignments_gate_fkey;
ALTER TABLE approved_assignments
    ADD CONSTRAINT approved_assignments_gate_fkey
    FOREIGN KEY (gate) REFERENCES gates(code) ON UPDATE CASCADE;

ALTER TABLE schedule_assignments DROP CONSTRAINT schedule_assignments_gate_fkey;
ALTER TABLE schedule_assignments
    ADD CONSTRAINT schedule_assignments_gate_fkey
    FOREIGN KEY (gate) REFERENCES gates(code) ON UPDATE CASCADE;

ALTER TABLE schedule_shortages DROP CONSTRAINT schedule_shortages_gate_fkey;
ALTER TABLE schedule_shortages
    ADD CONSTRAINT schedule_shortages_gate_fkey
    FOREIGN KEY (gate) REFERENCES gates(code) ON UPDATE CASCADE;
