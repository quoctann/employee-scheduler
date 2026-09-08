package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
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
