package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/config"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/copilot"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/httpapi"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/management"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/observability"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/platform"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/secretbox"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
	postgresstore "github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("incident service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	settings, err := config.Load()
	if err != nil {
		return err
	}

	poolConfig, err := pgxpool.ParseConfig(settings.Database.URL)
	if err != nil {
		return err
	}
	poolConfig.MaxConns = settings.Database.MaxConnections
	poolConfig.MinConns = settings.Database.MinConnections
	poolConfig.MaxConnLifetime = settings.Database.MaxLifetime
	poolConfig.MaxConnIdleTime = settings.Database.MaxIdleTime
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return err
	}
	defer pool.Close()

	startupContext, startupCancel := context.WithTimeout(context.Background(), settings.Database.StartupTimeout)
	defer startupCancel()
	if err := pool.Ping(startupContext); err != nil {
		return err
	}
	if err := postgresstore.ApplyMigrations(startupContext, pool); err != nil {
		return err
	}
	credentialBox, err := secretbox.New(settings.EncryptionKey)
	if err != nil {
		return err
	}
	repository := postgresstore.New(pool, credentialBox)

	grpcConnection, err := grpc.NewClient(settings.AI.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer grpcConnection.Close()
	copilotClient := copilot.New(grpcConnection)
	platformClient, err := platform.NewConfigured(
		settings.Platform.GitHubAPIBase, settings.Platform.ConnectTimeout, settings.Platform.RequestTimeout,
		settings.Platform.AllowPrivateIntegrationEndpoints, platform.KubernetesConfig{
			Namespace: settings.Platform.KubernetesNamespace, CAFile: settings.Platform.KubernetesCAFile,
			TokenFile: settings.Platform.KubernetesTokenFile,
		},
	)
	if err != nil {
		return err
	}

	incidentService := service.NewIncidentService(repository, copilotClient, service.SystemClock, service.UUID, settings.AI.RPCTimeout)
	pipelineService := service.NewPipelineService(repository, service.SystemClock, service.UUID, settings.Platform.DefaultJenkins, settings.Platform.DefaultJob)
	automation := service.NewAutomationRunner(repository, platformClient, incidentService, pipelineService,
		service.SystemClock, service.AutomationConfig{
			SweepInterval: settings.Automation.SweepInterval, BuildPollInterval: settings.Automation.BuildPollInterval,
			BuildTimeout: settings.Automation.BuildTimeout, MaxSourceFiles: settings.Automation.MaxSourceFiles,
			MaxLogBytes: settings.Automation.MaxLogBytes, MaxSourceBytes: settings.Automation.MaxSourceBytes,
		}, logger)
	integrationService := service.NewIntegrationService(repository, platformClient, service.SystemClock, service.IntegrationDefaults{
		Username: "RamanRed", GitHubRepo: settings.Platform.DefaultRepo,
		GitHubBranch: settings.Platform.DefaultBranch, JenkinsURL: settings.Platform.DefaultJenkins,
		JenkinsJob: settings.Platform.DefaultJob,
	}, service.WithIntegrationConnector(platformClient), service.WithIntegrationSynchronizer(automation))
	authService, err := service.NewConfiguredAuthService(
		settings.Security.SessionSecret, settings.Security.BootstrapPassword,
		settings.Security.BootstrapRole, settings.Security.DemoMode,
		service.SystemClock, settings.Security.SessionTTL,
	)
	if err != nil {
		return err
	}

	metrics := observability.New(settings.ApplicationName, pool)
	managementHandler := management.New(repository, metrics.Handler(), settings.ApplicationName, settings.Version)
	apiHandler := httpapi.New(httpapi.Dependencies{
		Incidents: incidentService, Pipelines: pipelineService, Integrations: integrationService,
		Auth: authService, Management: managementHandler, Clock: service.SystemClock,
		Logger: logger, Version: settings.Version, RequireAuth: !settings.Security.DemoMode,
		AllowedOrigins: settings.Security.AllowedOrigins,
		CIWebhookToken: settings.Security.CIWebhookToken,
	})
	server := &http.Server{
		Addr: settings.HTTP.Address, Handler: metrics.Middleware(apiHandler),
		ReadTimeout: settings.HTTP.ReadTimeout, WriteTimeout: settings.HTTP.WriteTimeout,
		IdleTimeout: settings.HTTP.IdleTimeout,
	}

	shutdownSignal, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	automationDone := make(chan struct{})
	if settings.Automation.Enabled {
		go func() {
			defer close(automationDone)
			automation.Run(shutdownSignal)
		}()
	} else {
		close(automationDone)
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("incident service listening", "address", settings.HTTP.Address, "ai_target", settings.AI.Target)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case serverError := <-serverErrors:
		if !errors.Is(serverError, http.ErrServerClosed) {
			return serverError
		}
		return nil
	case <-shutdownSignal.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), settings.HTTP.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			_ = server.Close()
			return err
		}
		select {
		case <-automationDone:
		case <-shutdownContext.Done():
			return shutdownContext.Err()
		}
		logger.Info("incident service shutdown complete")
		return nil
	}
}
