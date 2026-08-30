package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/config"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/grpcapi"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/llm"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/management"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/metrics"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/remediation"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/triage"
	"github.com/RamanRed/SRE-DETECTION/gen/copilotpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("AI copilot service stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	var provider llm.Provider
	if cfg.ProviderEnabled() {
		openAIProvider, err := llm.NewOpenAIClient(llm.OpenAIConfig{
			APIKey:      cfg.OpenAIAPIKey,
			BaseURL:     cfg.OpenAIBaseURL,
			Model:       cfg.OpenAIModel,
			Temperature: cfg.OpenAITemperature,
			Timeout:     cfg.ProviderTimeout,
		})
		if err != nil {
			return err
		}
		provider = openAIProvider
		logger.Info("OpenAI-compatible provider enabled", "base_url", cfg.OpenAIBaseURL, "model", cfg.OpenAIModel)
	} else {
		logger.Info("AI provider disabled; heuristic triage is active")
	}

	latency := metrics.NewInferenceHistogram(cfg.ApplicationName)
	analyzer := triage.NewAnalyzer(provider, logger)
	remediationGenerator := remediation.NewGenerator(cfg.RemediationNamespace)
	service := grpcapi.NewServer(analyzer, remediationGenerator, latency, logger)

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		return err
	}
	managementListener, err := net.Listen("tcp", cfg.ManagementAddress)
	if err != nil {
		_ = grpcListener.Close()
		return err
	}

	grpcServer := grpc.NewServer()
	copilotpb.RegisterIncidentCopilotServiceServer(grpcServer, service)
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(copilotpb.IncidentCopilotService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)

	managementHandler := management.NewHandler(cfg.ApplicationName, latency)
	httpServer := &http.Server{
		Handler:           managementHandler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	managementHandler.SetReady(true)
	errCh := make(chan error, 2)
	go func() {
		logger.Info("gRPC server listening", "address", grpcListener.Addr().String())
		if serveErr := grpcServer.Serve(grpcListener); serveErr != nil {
			errCh <- serveErr
		}
	}()
	go func() {
		logger.Info("management server listening", "address", managementListener.Addr().String())
		if serveErr := httpServer.Serve(managementListener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case serveErr = <-errCh:
		logger.Error("server stopped unexpectedly", "error", serveErr)
	}

	managementHandler.SetReady(false)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	healthServer.SetServingStatus(copilotpb.IncidentCopilotService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	var shutdownGroup sync.WaitGroup
	shutdownGroup.Add(2)
	go func() {
		defer shutdownGroup.Done()
		if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			logger.Warn("management server shutdown failed", "error", err)
		}
	}()
	go func() {
		defer shutdownGroup.Done()
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-shutdownCtx.Done():
			grpcServer.Stop()
		}
	}()
	shutdownGroup.Wait()
	logger.Info("AI copilot service stopped")
	return serveErr
}
