package config

import (
	"strings"
	"testing"
)

func TestLoadHonorsAIEndpointAndUsesNeutralIntegrationDefaults(t *testing.T) {
	t.Setenv("AI_COPILOT_HOST", "copilot.internal")
	t.Setenv("AI_COPILOT_PORT", "19090")
	t.Setenv("JENKINS_URL", "")
	t.Setenv("JENKINS_JOB_NAME", "")
	t.Setenv("APP_VERSION", "")
	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.AI.Target != "copilot.internal:19090" {
		t.Fatalf("AI target = %q", settings.AI.Target)
	}
	if settings.Platform.DefaultJenkins != "http://localhost:8080" || settings.Platform.DefaultJob != "sre-copilot-pipeline" {
		t.Fatalf("Jenkins defaults = %q %q", settings.Platform.DefaultJenkins, settings.Platform.DefaultJob)
	}
	if settings.Version != "1.0.0" {
		t.Fatalf("version = %q", settings.Version)
	}
}

func TestSecureModeRequiresSeparatedAuthAndEncryptionSecrets(t *testing.T) {
	t.Setenv("DEMO_MODE", "false")
	t.Setenv("AUTH_SESSION_SECRET", strings.Repeat("s", 32))
	t.Setenv("AUTH_BOOTSTRAP_PASSWORD", "correct horse battery")
	t.Setenv("AUTH_BOOTSTRAP_ROLE", "DEVOPS_ENGINEER")
	t.Setenv("INTEGRATION_ENCRYPTION_KEY", strings.Repeat("e", 32))
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://console.example")
	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Security.DemoMode || settings.Security.BootstrapRole != "DEVOPS_ENGINEER" || len(settings.Security.AllowedOrigins) != 1 {
		t.Fatalf("security settings = %+v", settings.Security)
	}
	t.Setenv("AUTH_BOOTSTRAP_ROLE", "ADMIN")
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted an unknown bootstrap role")
	}
	t.Setenv("AUTH_BOOTSTRAP_ROLE", "SRE_LEAD")
	t.Setenv("CORS_ALLOWED_ORIGINS", ",,,")
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted an empty secure CORS allowlist")
	}
}

func TestLoadAcceptsPreformattedAIHostAndRejectsPoolInversion(t *testing.T) {
	t.Setenv("AI_COPILOT_HOST", "dns:///copilot:9090")
	t.Setenv("DB_MIN_CONNS", "8")
	t.Setenv("DB_MAX_CONNS", "4")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted inverted database pool bounds")
	}
	t.Setenv("DB_MIN_CONNS", "2")
	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.AI.Target != "dns:///copilot:9090" {
		t.Fatalf("AI target = %q", settings.AI.Target)
	}
}
