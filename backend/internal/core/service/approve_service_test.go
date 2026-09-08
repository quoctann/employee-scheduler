package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
)

func TestApproveService_Approve_DelegatesToRepository(t *testing.T) {
	repo := &fakeScheduleRepository{approveCount: 2}
	svc := NewApproveService(repo)
	assignments := []domain.LockedAssignment{
		{EmployeeID: "NV01", Date: mustDate(t, "2026-09-07"), Off: true},
	}

	count, err := svc.Approve(context.Background(), assignments)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("Approve() count = %d, want 2", count)
	}
	if len(repo.approved) != 1 {
		t.Fatalf("expected assignments forwarded to repository, got %+v", repo.approved)
	}
}

func TestApproveService_Approve_EmptyIsNoop(t *testing.T) {
	repo := &fakeScheduleRepository{}
	svc := NewApproveService(repo)

	count, err := svc.Approve(context.Background(), nil)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("Approve() count = %d, want 0", count)
	}
	if repo.approved != nil {
		t.Fatalf("expected repository not to be called for empty input, got %+v", repo.approved)
	}
}

func TestApproveService_Approve_PropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("db unavailable")
	repo := &fakeScheduleRepository{approveErr: wantErr}
	svc := NewApproveService(repo)

	_, err := svc.Approve(context.Background(), []domain.LockedAssignment{{EmployeeID: "NV01", Date: mustDate(t, "2026-09-07"), Off: true}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestApproveService_Approve_RejectsMalformedAssignmentBeforeHittingRepository(t *testing.T) {
	repo := &fakeScheduleRepository{}
	svc := NewApproveService(repo)

	// off=false but gate/shift both nil — violates the shape invariant.
	_, err := svc.Approve(context.Background(), []domain.LockedAssignment{
		{EmployeeID: "NV01", Date: mustDate(t, "2026-09-07"), Off: false},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if repo.approved != nil {
		t.Fatalf("expected repository not to be called for an invalid assignment, got %+v", repo.approved)
	}
}

func TestApproveService_Unapprove_DeletesHorizonAndReturnsCount(t *testing.T) {
	repo := &fakeScheduleRepository{unapproveCount: 5}
	svc := NewApproveService(repo)

	count, err := svc.Unapprove(context.Background(), HorizonParams{StartDate: mustDate(t, "2026-09-07"), NumDays: 3})
	if err != nil {
		t.Fatalf("Unapprove() error = %v", err)
	}
	if count != 5 {
		t.Fatalf("Unapprove() count = %d, want 5", count)
	}
	wantEnd := mustDate(t, "2026-09-09")
	if repo.unapproveFrom != mustDate(t, "2026-09-07") || repo.unapproveTo != wantEnd {
		t.Fatalf("expected horizon [2026-09-07,%v], got [%v,%v]", wantEnd, repo.unapproveFrom, repo.unapproveTo)
	}
}

func TestApproveService_Unapprove_RejectsInvalidNumDays(t *testing.T) {
	svc := NewApproveService(&fakeScheduleRepository{})
	if _, err := svc.Unapprove(context.Background(), HorizonParams{StartDate: mustDate(t, "2026-09-07"), NumDays: 0}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestApproveService_ListApproved_ReturnsRepositoryResult(t *testing.T) {
	locked := []domain.LockedAssignment{{EmployeeID: "NV01", Date: mustDate(t, "2026-09-07"), Off: true}}
	repo := &fakeScheduleRepository{approvedAssignments: locked}
	svc := NewApproveService(repo)

	got, err := svc.ListApproved(context.Background(), HorizonParams{StartDate: mustDate(t, "2026-09-07"), NumDays: 1})
	if err != nil {
		t.Fatalf("ListApproved() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected repository's approved assignments to be returned, got %+v", got)
	}
}
