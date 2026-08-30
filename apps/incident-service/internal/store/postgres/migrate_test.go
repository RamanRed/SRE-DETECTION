package postgres

import (
	"io/fs"
	"testing"
)

func TestEmbeddedMigrationsContainAnalysisAndNeutralDefaults(t *testing.T) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 10 {
		t.Fatalf("embedded migration count = %d, want 10", len(entries))
	}
	analysis, err := migrationFiles.ReadFile("migrations/V4__create_incident_analyses_table.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis) == 0 {
		t.Fatal("incident analysis migration is empty")
	}
	if _, err := migrationFiles.ReadFile("migrations/V5__neutralize_integration_defaults.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationFiles.ReadFile("migrations/V6__autonomous_integrations.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationFiles.ReadFile("migrations/V7__source_aware_incident_analysis.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationFiles.ReadFile("migrations/V8__commit_processing_state.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationFiles.ReadFile("migrations/V9__automation_idempotency.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationFiles.ReadFile("migrations/V10__scope_pipeline_build_identity.sql"); err != nil {
		t.Fatal(err)
	}
	if migrationNumber("V10__scope_pipeline_build_identity.sql") <= migrationNumber("V9__automation_idempotency.sql") {
		t.Fatal("numeric migration ordering does not place V10 after V9")
	}
}
