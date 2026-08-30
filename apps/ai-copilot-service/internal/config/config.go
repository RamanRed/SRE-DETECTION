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

const (
	defaultGRPCPort        = "9090"
	defaultManagementPort  = "8082"
	defaultOpenAIBaseURL   = "https://api.openai.com"
	defaultOpenAIModel     = "gpt-4o-mini"
	defaultTemperature     = 0.2
	defaultProviderTimeout = 15 * time.Second
	defaultShutdownTimeout = 10 * time.Second
	defaultRemediationNS   = "sre-copilot"
	defaultApplicationName = "ai-copilot-service"
)

// Config contains all runtime configuration for the AI copilot process.
// Native Go-oriented names take precedence over the legacy Spring names so
// deployments can migrate without a flag day.
type Config struct {
	ApplicationName      string
	GRPCAddress          string
	ManagementAddress    string
	OpenAIAPIKey         string
	OpenAIBaseURL        string
	OpenAIModel          string
	OpenAITemperature    float64
	ProviderTimeout      time.Duration
	ShutdownTimeout      time.Duration
	RemediationNamespace string
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

type lookupFunc func(string) (string, bool)

func load(lookup lookupFunc) (Config, error) {
	grpcAddress, err := listenAddress(
		lookup,
		[]string{"GRPC_ADDRESS", "AI_COPILOT_GRPC_ADDRESS"},
		[]string{"GRPC_PORT", "GRPC_SERVER_PORT"},
		defaultGRPCPort,
	)
	if err != nil {
		return Config{}, fmt.Errorf("gRPC listen address: %w", err)
	}

	managementAddress, err := listenAddress(
		lookup,
		[]string{"MANAGEMENT_ADDRESS", "HTTP_ADDRESS"},
		[]string{"MANAGEMENT_PORT", "HTTP_PORT", "SERVER_PORT"},
		defaultManagementPort,
	)
	if err != nil {
		return Config{}, fmt.Errorf("management listen address: %w", err)
	}

	temperature, err := floatValue(lookup, defaultTemperature,
		"OPENAI_TEMPERATURE",
		"AI_TEMPERATURE",
		"SPRING_AI_OPENAI_CHAT_OPTIONS_TEMPERATURE",
	)
	if err != nil {
		return Config{}, err
	}
	if temperature < 0 || temperature > 2 {
		return Config{}, fmt.Errorf("OpenAI temperature must be between 0 and 2, got %g", temperature)
	}

	providerTimeout, err := durationValue(lookup, defaultProviderTimeout,
		"OPENAI_TIMEOUT",
		"AI_PROVIDER_TIMEOUT",
		"LLM_TIMEOUT",
	)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationValue(lookup, defaultShutdownTimeout, "SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	baseURL := firstNonBlank(lookup,
		"OPENAI_BASE_URL",
		"AI_BASE_URL",
		"SPRING_AI_OPENAI_BASE_URL",
	)
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	if err := validateHTTPURL(baseURL); err != nil {
		return Config{}, fmt.Errorf("OpenAI base URL: %w", err)
	}

	namespace := firstNonBlank(lookup, "REMEDIATION_NAMESPACE")
	if namespace == "" {
		namespace = defaultRemediationNS
	}
	if !isDNSLabel(namespace) {
		return Config{}, fmt.Errorf("REMEDIATION_NAMESPACE %q is not a valid Kubernetes namespace", namespace)
	}

	model := firstNonBlank(lookup,
		"OPENAI_MODEL",
		"AI_MODEL",
		"SPRING_AI_OPENAI_CHAT_OPTIONS_MODEL",
		// Kept for the old Docker Compose example, which used this shorter key.
		"SPRING_AI_OPENAI_MODEL",
	)
	if model == "" {
		model = defaultOpenAIModel
	}

	applicationName := firstNonBlank(lookup, "APPLICATION_NAME", "SPRING_APPLICATION_NAME")
	if applicationName == "" {
		applicationName = defaultApplicationName
	}

	return Config{
		ApplicationName:   applicationName,
		GRPCAddress:       grpcAddress,
		ManagementAddress: managementAddress,
		OpenAIAPIKey: firstNonBlank(lookup,
			"OPENAI_API_KEY",
			"AI_API_KEY",
			"SPRING_AI_OPENAI_API_KEY",
		),
		OpenAIBaseURL:        strings.TrimRight(baseURL, "/"),
		OpenAIModel:          model,
		OpenAITemperature:    temperature,
		ProviderTimeout:      providerTimeout,
		ShutdownTimeout:      shutdownTimeout,
		RemediationNamespace: namespace,
	}, nil
}

// ProviderEnabled reports whether a real provider credential was supplied.
// The old Java service defaulted to demo-key, which should never trigger a
// network call in the Go service.
func (c Config) ProviderEnabled() bool {
	key := strings.ToLower(strings.TrimSpace(c.OpenAIAPIKey))
	return key != "" && key != "demo" && key != "demo-key"
}

func listenAddress(lookup lookupFunc, addressKeys, portKeys []string, defaultPort string) (string, error) {
	if address := firstNonBlank(lookup, addressKeys...); address != "" {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return "", fmt.Errorf("invalid address %q: %w", address, err)
		}
		return address, nil
	}

	port := firstNonBlank(lookup, portKeys...)
	if port == "" {
		port = defaultPort
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", fmt.Errorf("invalid port %q", port)
	}
	return net.JoinHostPort("", strconv.Itoa(portNumber)), nil
}

func firstNonBlank(lookup lookupFunc, keys ...string) string {
	for _, key := range keys {
		if value, ok := lookup(key); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func floatValue(lookup lookupFunc, fallback float64, keys ...string) (float64, error) {
	value := firstNonBlank(lookup, keys...)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", keys[0], err)
	}
	return parsed, nil
}

func durationValue(lookup lookupFunc, fallback time.Duration, keys ...string) (time.Duration, error) {
	value := firstNonBlank(lookup, keys...)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a Go duration such as 15s: %w", keys[0], err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", keys[0])
	}
	return parsed, nil
}

func validateHTTPURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("must be an absolute http or https URL")
	}
	return nil
}

func isDNSLabel(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	for index, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || (char == '-' && index > 0 && index < len(value)-1) {
			continue
		}
		return false
	}
	return true
}
