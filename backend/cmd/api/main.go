// Command api is the Go backend's entrypoint: it wires the Postgres and
// solver-service adapters into the core services and serves the minimal
// HTTP contract described in the project plan. No auth — demo scope.
//
// Besides serving HTTP (the default, no subcommand needed — matches the
// Dockerfile's ENTRYPOINT ["/app/api"]), it also exposes a "migrate"
// subcommand ("up"/"down"/"version") for manual migration ops.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"

	"github.com/quoctann/employee-scheduler-backend/internal/adapter/httpapi"
	"github.com/quoctann/employee-scheduler-backend/internal/adapter/postgres"
	"github.com/quoctann/employee-scheduler-backend/internal/adapter/solverclient"
	"github.com/quoctann/employee-scheduler-backend/internal/adapter/xlsxexport"
	"github.com/quoctann/employee-scheduler-backend/internal/config"
	"github.com/quoctann/employee-scheduler-backend/internal/core/service"
	"github.com/quoctann/employee-scheduler-backend/internal/platform/logging"
	"github.com/quoctann/employee-scheduler-backend/internal/platform/migrate"
	"github.com/quoctann/employee-scheduler-backend/internal/platform/tracing"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "api",
		Short: "Employee scheduler backend API server",
		// No subcommand -> serve HTTP, so the Dockerfile's plain
		// ENTRYPOINT ["/app/api"] keeps working unchanged.
		RunE: func(cmd *cobra.Command, args []string) error {
			runServer()
			return nil
		},
		SilenceUsage: true, // runtime errors (bad DB URL, etc.) shouldn't dump a usage block
	}
	root.AddCommand(newMigrateCmd())
	return root
}

// newMigrateCmd implements the manual `api migrate up|down|version` ops
// subcommand — independent of DB_AUTO_MIGRATE, which runs migrate.Up at
// server startup instead.
func newMigrateCmd() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	migrateCmd.AddCommand(
		&cobra.Command{
			Use:   "up",
			Short: "Apply all pending migrations",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				return migrate.Up(cfg.DatabaseURL)
			},
		},
		&cobra.Command{
			Use:   "down",
			Short: "Roll back all migrations",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				return migrate.Down(cfg.DatabaseURL)
			},
		},
		&cobra.Command{
			Use:   "version",
			Short: "Print the current schema version",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				version, dirty, err := migrate.Version(cfg.DatabaseURL)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "version=%d dirty=%t\n", version, dirty)
				return nil
			},
		},
	)
	return migrateCmd
}

func runServer() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger, err := logging.New(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck // best-effort flush on exit

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Init(ctx, tracing.Params{
		Disabled:    cfg.OTelSDKDisabled,
		Endpoint:    cfg.OTelExporterOTLPEndpoint,
		ServiceName: cfg.OTelServiceName,
	})
	if err != nil {
		logger.Fatal("init tracing", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := shutdownTracing(shutdownCtx); shutdownErr != nil {
			logger.Error("shutdown tracing", zap.Error(shutdownErr))
		}
	}()

	if cfg.DBAutoMigrate {
		logger.Info("running database migrations")
		if migrateErr := migrate.Up(cfg.DatabaseURL); migrateErr != nil {
			logger.Fatal("run migrations", zap.Error(migrateErr))
		}
	}

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, otelpgx.NewTracer())
	if err != nil {
		logger.Fatal("connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	employeeRepo := postgres.NewEmployeeRepository(pool)
	configRepo := postgres.NewConfigRepository(pool)
	scheduleRepo := postgres.NewScheduleRepository(pool)

	solverGateway := solverclient.New(cfg.SolverBaseURL, cfg.SolverAPIKey, otelhttp.NewTransport(http.DefaultTransport))

	srv := &httpapi.Server{
		Logger:     logger,
		Employees:  service.NewEmployeeService(employeeRepo),
		Config:     service.NewConfigService(configRepo),
		Schedule:   service.NewScheduleService(solverGateway, employeeRepo, configRepo, scheduleRepo),
		Approve:    service.NewApproveService(scheduleRepo),
		Capacity:   service.NewCapacityService(solverGateway, employeeRepo, configRepo),
		Candidates: service.NewCandidateService(solverGateway, employeeRepo, configRepo, scheduleRepo),
		Export:     service.NewExportService(employeeRepo, configRepo, scheduleRepo, xlsxexport.New()),
	}

	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      httpapi.NewRouter(srv, cfg.CORSOrigin, logger),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Minute, // solves can legitimately run long
	}

	go func() {
		logger.Info("listening", zap.String("addr", httpServer.Addr), zap.String("solver", cfg.SolverBaseURL))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("http server", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	}
}
