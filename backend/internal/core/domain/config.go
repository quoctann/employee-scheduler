package domain

// GateShiftRequirement mirrors solver-service's GateShiftRequirement.
type GateShiftRequirement struct {
	NV                int  `json:"nv"`
	Lead              int  `json:"lead"`
	LeadMandatoryRole bool `json:"lead_mandatory_role"`
}

// SolverWeights mirrors solver-service's SolverWeights defaults.
type SolverWeights struct {
	ShortfallPenalty     int `json:"shortfall_penalty"`
	LeadShortfallPenalty int `json:"lead_shortfall_penalty"`
	BalancePenaltyWeight int `json:"balance_penalty_weight"`
	StreakPenaltyWeight  int `json:"streak_penalty_weight"`
	StreakLength         int `json:"streak_length"`
}

// SolverConfig mirrors solver-service's SolverConfig: per-gate, per-shift
// requirements and durations, which gates need a shift lead, weekly target
// hours, and objective weights. This is business config, not hardcoded logic —
// it is loaded from the `gates` / `gate_shift_requirements` / `solver_settings`
// tables via ConfigRepository, not compiled in.
type SolverConfig struct {
	Requirements       map[string]map[ShiftType]GateShiftRequirement `json:"requirements"`
	ShiftHours         map[string]map[ShiftType]int                  `json:"shift_hours"`
	LeadGates          []string                                      `json:"lead_gates"`
	TargetHoursPerWeek int                                           `json:"target_hours_per_week"`
	Weights            SolverWeights                                 `json:"weights"`
}
