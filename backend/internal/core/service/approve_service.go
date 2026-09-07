package service

import (
	"context"
	"fmt"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
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
