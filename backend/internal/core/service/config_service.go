package service

import (
	"context"
	"fmt"

	"github.com/tantq/employee-scheduler-backend/internal/core/domain"
	"github.com/tantq/employee-scheduler-backend/internal/core/port"
)

// ConfigService exposes read-only scheduling configuration to inbound adapters.
type ConfigService struct {
	Config port.ConfigRepository
}

func NewConfigService(config port.ConfigRepository) *ConfigService {
	return &ConfigService{Config: config}
}

func (s *ConfigService) Get(ctx context.Context) (domain.SolverConfig, error) {
	config, err := s.Config.Get(ctx)
	if err != nil {
		return domain.SolverConfig{}, fmt.Errorf("load config: %w", err)
	}
	return config, nil
}
