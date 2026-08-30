package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(mapLookup(nil))
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if cfg.GRPCAddress != ":9090" {
		t.Fatalf("gRPC address = %q, want :9090", cfg.GRPCAddress)
	}
	if cfg.ManagementAddress != ":8082" {
		t.Fatalf("management address = %q, want :8082", cfg.ManagementAddress)
	}
	if cfg.OpenAIBaseURL != "https://api.openai.com" || cfg.OpenAIModel != "gpt-4o-mini" {
		t.Fatalf("unexpected OpenAI defaults: %+v", cfg)
	}
	if cfg.RemediationNamespace != "sre-copilot" {
		t.Fatalf("namespace = %q", cfg.RemediationNamespace)
	}
	if cfg.ProviderEnabled() {
		t.Fatal("provider must be disabled without a key")
	}
}

func TestLoadSupportsLegacySpringVariables(t *testing.T) {
	cfg, err := load(mapLookup(map[string]string{
		"GRPC_SERVER_PORT":                          "19090",
		"SERVER_PORT":                               "18082",
		"SPRING_AI_OPENAI_API_KEY":                  "legacy-key",
		"SPRING_AI_OPENAI_BASE_URL":                 "https://api.groq.test/openai/",
		"SPRING_AI_OPENAI_CHAT_OPTIONS_MODEL":       "legacy-model",
		"SPRING_AI_OPENAI_CHAT_OPTIONS_TEMPERATURE": "0.7",
	}))
	if err != nil {
		t.Fatalf("load legacy config: %v", err)
	}
	if cfg.GRPCAddress != ":19090" || cfg.ManagementAddress != ":18082" {
		t.Fatalf("legacy ports were not loaded: %+v", cfg)
	}
	if cfg.OpenAIAPIKey != "legacy-key" || cfg.OpenAIBaseURL != "https://api.groq.test/openai" || cfg.OpenAIModel != "legacy-model" {
		t.Fatalf("legacy OpenAI config was not loaded: %+v", cfg)
	}
	if cfg.OpenAITemperature != 0.7 || !cfg.ProviderEnabled() {
		t.Fatalf("unexpected provider state: %+v", cfg)
	}
}

func TestNativeVariablesTakePrecedence(t *testing.T) {
	cfg, err := load(mapLookup(map[string]string{
		"OPENAI_API_KEY":           "native-key",
		"SPRING_AI_OPENAI_API_KEY": "legacy-key",
		"OPENAI_MODEL":             "native-model",
		"SPRING_AI_OPENAI_MODEL":   "legacy-model",
		"OPENAI_TIMEOUT":           "3s",
		"REMEDIATION_NAMESPACE":    "platform",
		"GRPC_ADDRESS":             "127.0.0.1:9999",
		"GRPC_SERVER_PORT":         "19090",
		"MANAGEMENT_ADDRESS":       "127.0.0.1:8888",
	}))
	if err != nil {
		t.Fatalf("load native config: %v", err)
	}
	if cfg.OpenAIAPIKey != "native-key" || cfg.OpenAIModel != "native-model" {
		t.Fatalf("native variables did not win: %+v", cfg)
	}
	if cfg.ProviderTimeout != 3*time.Second || cfg.RemediationNamespace != "platform" {
		t.Fatalf("native options not loaded: %+v", cfg)
	}
	if cfg.GRPCAddress != "127.0.0.1:9999" || cfg.ManagementAddress != "127.0.0.1:8888" {
		t.Fatalf("explicit addresses not loaded: %+v", cfg)
	}
}

func TestDemoKeysDisableProvider(t *testing.T) {
	for _, key := range []string{"", "demo", "demo-key", " DEMO-KEY "} {
		cfg := Config{OpenAIAPIKey: key}
		if cfg.ProviderEnabled() {
			t.Errorf("key %q enabled provider", key)
		}
	}
	if !(Config{OpenAIAPIKey: "real-key"}).ProviderEnabled() {
		t.Fatal("real key did not enable provider")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "port", env: map[string]string{"GRPC_PORT": "70000"}},
		{name: "temperature", env: map[string]string{"OPENAI_TEMPERATURE": "3"}},
		{name: "timeout", env: map[string]string{"OPENAI_TIMEOUT": "never"}},
		{name: "base URL", env: map[string]string{"OPENAI_BASE_URL": "api.example.test"}},
		{name: "namespace", env: map[string]string{"REMEDIATION_NAMESPACE": "Not Valid"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := load(mapLookup(test.env)); err == nil {
				t.Fatal("load succeeded, want an error")
			}
		})
	}
}

func mapLookup(values map[string]string) lookupFunc {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
