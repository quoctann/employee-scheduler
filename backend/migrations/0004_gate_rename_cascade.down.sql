ALTER TABLE schedule_shortages DROP CONSTRAINT schedule_shortages_gate_fkey;
ALTER TABLE schedule_shortages
    ADD CONSTRAINT schedule_shortages_gate_fkey FOREIGN KEY (gate) REFERENCES gates(code);

ALTER TABLE schedule_assignments DROP CONSTRAINT schedule_assignments_gate_fkey;
ALTER TABLE schedule_assignments
    ADD CONSTRAINT schedule_assignments_gate_fkey FOREIGN KEY (gate) REFERENCES gates(code);

ALTER TABLE approved_assignments DROP CONSTRAINT approved_assignments_gate_fkey;
ALTER TABLE approved_assignments
    ADD CONSTRAINT approved_assignments_gate_fkey FOREIGN KEY (gate) REFERENCES gates(code);

ALTER TABLE gate_shift_requirements DROP CONSTRAINT gate_shift_requirements_gate_code_fkey;
ALTER TABLE gate_shift_requirements
    ADD CONSTRAINT gate_shift_requirements_gate_code_fkey FOREIGN KEY (gate_code) REFERENCES gates(code);
