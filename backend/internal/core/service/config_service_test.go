package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
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
