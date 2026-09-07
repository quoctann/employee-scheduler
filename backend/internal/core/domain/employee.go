package domain

// Role mirrors solver-service's Role literal: "NV" (staff), "TC" (shift lead),
// "PC" (deputy lead).
type Role string

const (
	RoleNV Role = "NV"
	RoleTC Role = "TC"
	RolePC Role = "PC"
)

// ShiftType mirrors solver-service's ShiftType literal.
type ShiftType string

const (
	ShiftSang ShiftType = "sang" // morning
	ShiftDem  ShiftType = "dem"  // night
)

var ShiftTypes = [...]ShiftType{ShiftSang, ShiftDem}

type Employee struct {
	EmployeeID string `json:"employee_id"`
	Name       string `json:"name"`
	Role       Role   `json:"role"`
	Active     bool   `json:"active"`
	LeaveDays  []Date `json:"leave_days,omitempty"`
}

type ShiftAvailability struct {
	Sang bool `json:"sang"`
	Dem  bool `json:"dem"`
}

// AvailabilityMap: employee_id -> date -> availability, matching solver-service's
// AvailabilityMap shape exactly (nested map keyed by date, not a flat list).
type AvailabilityMap map[string]map[Date]ShiftAvailability
