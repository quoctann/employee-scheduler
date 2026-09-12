package service

import (
	"context"
	"fmt"

	"github.com/quoctann/employee-scheduler-backend/internal/core/domain"
	"github.com/quoctann/employee-scheduler-backend/internal/core/port"
)

type ApproveService struct {
	Schedules port.ScheduleRepository
}

func NewApproveService(schedules port.ScheduleRepository) *ApproveService {
	return &ApproveService{Schedules: schedules}
}

// Approve records the manager-confirmed state for each cell, so future
// solves treat them as locked and rolling-horizon carry-in reflects them.
func (s *ApproveService) Approve(ctx context.Context, assignments []domain.LockedAssignment) (int, error) {
	if len(assignments) == 0 {
		return 0, nil
	}
	for _, a := range assignments {
		if err := a.Validate(); err != nil {
			return 0, fmt.Errorf("%w: %s", ErrInvalidInput, err)
		}
	}
	count, err := s.Schedules.ApproveAssignments(ctx, assignments)
	if err != nil {
		return 0, fmt.Errorf("approve assignments: %w", err)
	}
	return count, nil
}

// HorizonParams identifies a [StartDate, StartDate+NumDays-1] date range,
// shared by Unapprove and ListApproved (both operate over the same kind of
// horizon a solve does).
type HorizonParams struct {
	StartDate domain.Date
	NumDays   int
}

// Unapprove deletes the approved cells in the given horizon, so a future
// solve — even in "only unapproved" mode — is free to reassign them.
func (s *ApproveService) Unapprove(ctx context.Context, params HorizonParams) (int, error) {
	if params.NumDays < 1 {
		return 0, fmt.Errorf("%w: num_days must be >= 1, got %d", ErrInvalidInput, params.NumDays)
	}
	endDate := params.StartDate.AddDays(params.NumDays - 1)
	count, err := s.Schedules.UnapproveAssignments(ctx, params.StartDate, endDate)
	if err != nil {
		return 0, fmt.Errorf("unapprove assignments: %w", err)
	}
	return count, nil
}

// ListApproved returns the currently-approved cells in the given horizon, so
// the UI can highlight which cells are locked in.
func (s *ApproveService) ListApproved(ctx context.Context, params HorizonParams) ([]domain.LockedAssignment, error) {
	if params.NumDays < 1 {
		return nil, fmt.Errorf("%w: num_days must be >= 1, got %d", ErrInvalidInput, params.NumDays)
	}
	endDate := params.StartDate.AddDays(params.NumDays - 1)
	assignments, err := s.Schedules.ApprovedAssignments(ctx, params.StartDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("list approved assignments: %w", err)
	}
	return assignments, nil
}
