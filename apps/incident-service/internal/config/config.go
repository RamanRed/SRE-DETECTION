package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ApplicationName string
	Version         string
	HTTP            HTTPConfig
	Database        DatabaseConfig
	AI              AIConfig
	Platform        PlatformConfig
	Automation      AutomationConfig
	EncryptionKey   string
	Security        SecurityConfig
}

type HTTPConfig struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	URL            string
	MaxConnections int32
	MinConnections int32
	MaxLifetime    time.Duration
	MaxIdleTime    time.Duration
	StartupTimeout time.Duration
}

type AIConfig struct {
	Target     string
	RPCTimeout time.Duration
}

type PlatformConfig struct {
	GitHubAPIBase                    string
	DefaultRepo                      string
	DefaultBranch                    string
	DefaultJenkins                   string
	DefaultJob                       string
	ConnectTimeout                   time.Duration
	RequestTimeout                   time.Duration
	AllowPrivateIntegrationEndpoints bool
	KubernetesNamespace              string
	KubernetesCAFile                 string
	KubernetesTokenFile              string
}

type AutomationConfig struct {
	Enabled           bool
	SweepInterval     time.Duration
	BuildPollInterval time.Duration
	BuildTimeout      time.Duration
	MaxSourceFiles    int
	MaxLogBytes       int
	MaxSourceBytes    int
}

type SecurityConfig struct {
	DemoMode          bool
	SessionSecret     string
	BootstrapPassword string
	BootstrapRole     string
	CIWebhookToken    string
	SessionTTL        time.Duration
	AllowedOrigins    []string
}

func Load() (Config, error) {
	serverPort := env("SERVER_PORT", "8081")
	if _, err := strconv.ParseUint(serverPort, 10, 16); err != nil {
		return Config{}, fmt.Errorf("SERVER_PORT must be a valid TCP port: %w", err)
	}

	readTimeout, err := duration("HTTP_READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := duration("HTTP_WRITE_TIMEOUT", 35*time.Second)
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := duration("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	dbStartupTimeout, err := duration("DB_CONNECT_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	aiTimeout, err := duration("AI_RPC_TIMEOUT", 20*time.Second)
	if err != nil {
		return Config{}, err
	}
	platformConnectTimeout, err := duration("PLATFORM_CONNECT_TIMEOUT", 6*time.Second)
	if err != nil {
		return Config{}, err
	}
	platformRequestTimeout, err := duration("PLATFORM_REQUEST_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	sweepInterval, err := duration("AUTOMATION_SWEEP_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	buildPollInterval, err := duration("CI_STATUS_POLL_INTERVAL", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	buildTimeout, err := duration("CI_BUILD_TIMEOUT", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	automationEnabled, err := boolean("AUTOMATION_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	allowPrivateEndpoints, err := boolean("ALLOW_PRIVATE_INTEGRATION_ENDPOINTS", false)
	if err != nil {
		return Config{}, err
	}
	requireEncryptionKey, err := boolean("REQUIRE_INTEGRATION_ENCRYPTION_KEY", false)
	if err != nil {
		return Config{}, err
	}
	encryptionKey := strings.TrimSpace(os.Getenv("INTEGRATION_ENCRYPTION_KEY"))
	if requireEncryptionKey && encryptionKey == "" {
		return Config{}, fmt.Errorf("INTEGRATION_ENCRYPTION_KEY is required when REQUIRE_INTEGRATION_ENCRYPTION_KEY=true")
	}
	demoMode, err := boolean("DEMO_MODE", true)
	if err != nil {
		return Config{}, err
	}
	if !demoMode && encryptionKey == "" {
		return Config{}, fmt.Errorf("INTEGRATION_ENCRYPTION_KEY is required when DEMO_MODE=false")
	}
	sessionTTL, err := duration("AUTH_SESSION_TTL", 8*time.Hour)
	if err != nil {
		return Config{}, err
	}
	sessionSecret := os.Getenv("AUTH_SESSION_SECRET")
	bootstrapPassword := os.Getenv("AUTH_BOOTSTRAP_PASSWORD")
	bootstrapRole := env("AUTH_BOOTSTRAP_ROLE", "SRE_LEAD")
	bootstrapRole = strings.ToUpper(bootstrapRole)
	switch bootstrapRole {
	case "SRE_LEAD", "DEVOPS_ENGINEER", "EVALUATOR":
	default:
		return Config{}, fmt.Errorf("AUTH_BOOTSTRAP_ROLE must be SRE_LEAD, DEVOPS_ENGINEER, or EVALUATOR")
	}
	if !demoMode && len(sessionSecret) < 32 {
		return Config{}, fmt.Errorf("AUTH_SESSION_SECRET must be at least 32 bytes when DEMO_MODE=false")
	}
	if !demoMode && len(bootstrapPassword) < 12 {
		return Config{}, fmt.Errorf("AUTH_BOOTSTRAP_PASSWORD must be at least 12 bytes when DEMO_MODE=false")
	}
	defaultOrigins := "*"
	if !demoMode {
		defaultOrigins = "http://localhost:3000,http://localhost:5173"
	}
	allowedOrigins := commaSeparated(env("CORS_ALLOWED_ORIGINS", defaultOrigins))
	if !demoMode {
		if len(allowedOrigins) == 0 {
			return Config{}, fmt.Errorf("CORS_ALLOWED_ORIGINS must contain at least one origin when DEMO_MODE=false")
		}
		for _, origin := range allowedOrigins {
			if origin == "*" {
				return Config{}, fmt.Errorf("CORS_ALLOWED_ORIGINS may not contain * when DEMO_MODE=false")
			}
		}
	}

	maxConnections, err := int32Value("DB_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	minConnections, err := int32Value("DB_MIN_CONNS", 2)
	if err != nil {
		return Config{}, err
	}
	if minConnections < 0 || maxConnections < 1 || minConnections > maxConnections {
		return Config{}, fmt.Errorf("database pool bounds are invalid: min=%d max=%d", minConnections, maxConnections)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = postgresURL(dbStartupTimeout)
	}

	aiHost := env("AI_COPILOT_HOST", "ai-copilot-service")
	aiPort := env("AI_COPILOT_PORT", "9090")
	aiTarget := aiHost
	if !strings.Contains(aiHost, "://") {
		if _, _, splitErr := net.SplitHostPort(aiHost); splitErr != nil {
			aiTarget = net.JoinHostPort(strings.Trim(aiHost, "[]"), aiPort)
		}
	}

	return Config{
		ApplicationName: "incident-service",
		Version:         env("APP_VERSION", "1.0.0"),
		HTTP: HTTPConfig{
			Address:         ":" + serverPort,
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			IdleTimeout:     idleTimeout,
			ShutdownTimeout: shutdownTimeout,
		},
		Database: DatabaseConfig{
			URL:            databaseURL,
			MaxConnections: maxConnections,
			MinConnections: minConnections,
			MaxLifetime:    30 * time.Minute,
			MaxIdleTime:    10 * time.Minute,
			StartupTimeout: dbStartupTimeout,
		},
		AI: AIConfig{
			Target:     aiTarget,
			RPCTimeout: aiTimeout,
		},
		Platform: PlatformConfig{
			GitHubAPIBase:                    env("GITHUB_API_BASE", "https://api.github.com"),
			DefaultRepo:                      env("GITHUB_REPO", "RamanRed/SRE-DETECTION"),
			DefaultBranch:                    env("GITHUB_BRANCH", "main"),
			DefaultJenkins:                   env("JENKINS_URL", "http://localhost:8080"),
			DefaultJob:                       env("JENKINS_JOB_NAME", "sre-copilot-pipeline"),
			ConnectTimeout:                   platformConnectTimeout,
			RequestTimeout:                   platformRequestTimeout,
			AllowPrivateIntegrationEndpoints: allowPrivateEndpoints,
			KubernetesNamespace:              env("KUBERNETES_NAMESPACE", "sre-copilot"),
			KubernetesCAFile:                 env("KUBERNETES_CA_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"),
			KubernetesTokenFile:              env("KUBERNETES_SERVICEACCOUNT_TOKEN_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/token"),
		},
		Automation: AutomationConfig{
			Enabled: automationEnabled, SweepInterval: sweepInterval,
			BuildPollInterval: buildPollInterval, BuildTimeout: buildTimeout,
			MaxSourceFiles: 5, MaxLogBytes: 64 << 10, MaxSourceBytes: 32 << 10,
		},
		EncryptionKey: encryptionKey,
		Security: SecurityConfig{
			DemoMode: demoMode, SessionSecret: sessionSecret,
			BootstrapPassword: bootstrapPassword, BootstrapRole: bootstrapRole,
			CIWebhookToken: os.Getenv("CI_WEBHOOK_TOKEN"),
			SessionTTL:     sessionTTL, AllowedOrigins: allowedOrigins,
		},
	}, nil
}

func commaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func postgresURL(connectTimeout time.Duration) string {
	dbURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(env("DB_USERNAME", "sreuser"), env("DB_PASSWORD", "srepassword")),
		Host:   net.JoinHostPort(env("DB_HOST", "localhost"), env("DB_PORT", "5432")),
		Path:   "/" + env("DB_NAME", "sredb"),
	}
	query := dbURL.Query()
	query.Set("sslmode", env("DB_SSLMODE", "disable"))
	query.Set("connect_timeout", strconv.Itoa(max(1, int(connectTimeout.Seconds()))))
	dbURL.RawQuery = query.Encode()
	return dbURL.String()
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return parsed, nil
}

func int32Value(name string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return int32(parsed), nil
}

func boolean(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false: %w", name, err)
	}
	return parsed, nil
}
