-- name: ListGates :many
SELECT code, is_lead_gate FROM gates ORDER BY code;

-- name: ListGateShiftRequirements :many
SELECT gate_code, shift_type, nv, lead, lead_mandatory_role, shift_hours
FROM gate_shift_requirements;

-- name: GetSolverSettings :one
SELECT target_hours_per_week, shortfall_penalty, lead_shortfall_penalty,
       balance_penalty_weight, streak_penalty_weight, streak_length
FROM solver_settings
WHERE id = 1;

-- name: UpdateGateShiftRequirement :execrows
UPDATE gate_shift_requirements
SET nv = $3, lead = $4, lead_mandatory_role = $5, shift_hours = $6
WHERE gate_code = $1 AND shift_type = $2;

-- name: RenameGate :execrows
UPDATE gates SET code = $2 WHERE code = $1;
