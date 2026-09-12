package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
)

func TestConfigService_Get(t *testing.T) {
	want := domain.SolverConfig{ShiftHours: map[string]map[domain.ShiftType]int{"A": {domain.ShiftSang: 11, domain.ShiftDem: 13}}}
	svc := NewConfigService(&fakeConfigRepository{config: want})
	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() = %+v, want %+v", got, want)
	}
}

func TestConfigService_GetWrapsRepositoryError(t *testing.T) {
	repoErr := errors.New("database unavailable")
	_, err := NewConfigService(&fakeConfigRepository{err: repoErr}).Get(context.Background())
	if !errors.Is(err, repoErr) {
		t.Fatalf("Get() error = %v, want wrapped %v", err, repoErr)
	}
}

func TestConfigService_UpdateGateShiftRequirement_HappyPath(t *testing.T) {
	repo := &fakeConfigRepository{}
	svc := NewConfigService(repo)

	got, err := svc.UpdateGateShiftRequirement(context.Background(), "B", domain.ShiftDem, domain.GateShiftRequirement{NV: 2, Lead: 1, LeadMandatoryRole: true}, 12)
	if err != nil {
		t.Fatalf("UpdateGateShiftRequirement() error = %v", err)
	}
	if repo.lastUpdateGate != "B" || repo.lastUpdateType != domain.ShiftDem || repo.lastUpdateHrs != 12 {
		t.Fatalf("repository received gate=%q shift=%q hours=%d, want B/dem/12", repo.lastUpdateGate, repo.lastUpdateType, repo.lastUpdateHrs)
	}
	if repo.lastUpdateReq != (domain.GateShiftRequirement{NV: 2, Lead: 1, LeadMandatoryRole: true}) {
		t.Fatalf("repository received requirement = %+v", repo.lastUpdateReq)
	}
	if got.Requirements["B"][domain.ShiftDem].Lead != 1 {
		t.Fatalf("Update() returned config = %+v, want it to reflect the update", got)
	}
}

func TestConfigService_UpdateGateShiftRequirement_ValidatesInput(t *testing.T) {
	cases := []struct {
		name       string
		gateCode   string
		shiftType  domain.ShiftType
		req        domain.GateShiftRequirement
		shiftHours int
	}{
		{"blank gate code", "  ", domain.ShiftSang, domain.GateShiftRequirement{}, 8},
		{"invalid shift type", "A", "afternoon", domain.GateShiftRequirement{}, 8},
		{"negative nv", "A", domain.ShiftSang, domain.GateShiftRequirement{NV: -1}, 8},
		{"nv over max", "A", domain.ShiftSang, domain.GateShiftRequirement{NV: 1001}, 8},
		{"negative lead", "A", domain.ShiftSang, domain.GateShiftRequirement{Lead: -1}, 8},
		{"shift hours too low", "A", domain.ShiftSang, domain.GateShiftRequirement{}, 0},
		{"shift hours too high", "A", domain.ShiftSang, domain.GateShiftRequirement{}, 25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewConfigService(&fakeConfigRepository{})
			_, err := svc.UpdateGateShiftRequirement(context.Background(), tc.gateCode, tc.shiftType, tc.req, tc.shiftHours)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("UpdateGateShiftRequirement() error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestConfigService_UpdateGateShiftRequirement_NotFoundMapsToErrNotFound(t *testing.T) {
	repo := &fakeConfigRepository{updateErr: port.ErrGateShiftNotFound}
	_, err := NewConfigService(repo).UpdateGateShiftRequirement(context.Background(), "Z", domain.ShiftSang, domain.GateShiftRequirement{}, 8)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateGateShiftRequirement() error = %v, want ErrNotFound", err)
	}
}

func TestConfigService_RenameGate_HappyPath(t *testing.T) {
	repo := &fakeConfigRepository{config: domain.SolverConfig{
		Requirements: map[string]map[domain.ShiftType]domain.GateShiftRequirement{"A": {domain.ShiftSang: {NV: 2}}},
		ShiftHours:   map[string]map[domain.ShiftType]int{"A": {domain.ShiftSang: 8}},
		LeadGates:    []string{"A"},
	}}
	svc := NewConfigService(repo)

	got, err := svc.RenameGate(context.Background(), "A", "A2")
	if err != nil {
		t.Fatalf("RenameGate() error = %v", err)
	}
	if repo.lastRenameOld != "A" || repo.lastRenameNew != "A2" {
		t.Fatalf("repository received old=%q new=%q, want A/A2", repo.lastRenameOld, repo.lastRenameNew)
	}
	if _, stillThere := got.Requirements["A"]; stillThere {
		t.Fatalf("RenameGate() left old code %q in Requirements: %+v", "A", got.Requirements)
	}
	if got.Requirements["A2"][domain.ShiftSang].NV != 2 {
		t.Fatalf("RenameGate() = %+v, want requirement to follow the new code", got.Requirements)
	}
	if len(got.LeadGates) != 1 || got.LeadGates[0] != "A2" {
		t.Fatalf("RenameGate() lead_gates = %v, want [A2]", got.LeadGates)
	}
}

func TestConfigService_RenameGate_ValidatesInput(t *testing.T) {
	cases := []struct {
		name    string
		oldCode string
		newCode string
	}{
		{"blank old code", "  ", "B"},
		{"blank new code", "A", "  "},
		{"new code has invalid characters", "A", "A/2"},
		{"new code too long", "A", strings.Repeat("x", maxGateCodeLength+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewConfigService(&fakeConfigRepository{})
			_, err := svc.RenameGate(context.Background(), tc.oldCode, tc.newCode)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("RenameGate() error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestConfigService_RenameGate_NotFoundMapsToErrNotFound(t *testing.T) {
	repo := &fakeConfigRepository{renameErr: port.ErrGateNotFound}
	_, err := NewConfigService(repo).RenameGate(context.Background(), "Z", "Y")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("RenameGate() error = %v, want ErrNotFound", err)
	}
}

func TestConfigService_RenameGate_ConflictMapsToErrConflict(t *testing.T) {
	repo := &fakeConfigRepository{renameErr: port.ErrGateConflict}
	_, err := NewConfigService(repo).RenameGate(context.Background(), "A", "B")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("RenameGate() error = %v, want ErrConflict", err)
	}
}
