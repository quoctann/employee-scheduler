// Command api is the Go backend's entrypoint: it wires the Postgres and
// solver-service adapters into the core services and serves the minimal
// HTTP contract described in the project plan. No auth — demo scope.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/tantq/employee-scheduler-backend/internal/adapter/httpapi"
	"github.com/tantq/employee-scheduler-backend/internal/adapter/postgres"
	"github.com/tantq/employee-scheduler-backend/internal/adapter/solverclient"
	"github.com/tantq/employee-scheduler-backend/internal/config"
	"github.com/tantq/employee-scheduler-backend/internal/core/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer pool.Close()

	employeeRepo := postgres.NewEmployeeRepository(pool)
	configRepo := postgres.NewConfigRepository(pool)
	scheduleRepo := postgres.NewScheduleRepository(pool)
	solverGateway := solverclient.New(cfg.SolverBaseURL, cfg.SolverAPIKey)

	srv := &httpapi.Server{
		Employees:  service.NewEmployeeService(employeeRepo),
		Config:     service.NewConfigService(configRepo),
		Schedule:   service.NewScheduleService(solverGateway, employeeRepo, configRepo, scheduleRepo),
		Approve:    service.NewApproveService(scheduleRepo),
		Capacity:   service.NewCapacityService(solverGateway, employeeRepo, configRepo),
		Candidates: service.NewCandidateService(solverGateway, employeeRepo, configRepo, scheduleRepo),
	}

	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      httpapi.NewRouter(srv, cfg.CORSOrigin),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Minute, // solves can legitimately run long
	}

	go func() {
		log.Printf("listening on %s (solver=%s)", httpServer.Addr, cfg.SolverBaseURL)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
