package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	storecontract "github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/store"
	"github.com/jackc/pgx/v5"
)

const incidentColumns = `id,title,service_name,raw_logs,firing_rule,environment,status,severity,created_by,created_at,updated_at,resolved_at,source_event_key`
const remediationColumns = `id,incident_id,ai_root_cause,suggested_action,script_type,executable_script,confidence_score,affected_components,requires_approval,applied_by,execution_status,created_at,applied_at,unified_diff,verification_plan,rollback_plan`

func (s *Store) CreateIncident(ctx context.Context, incident domain.Incident) (domain.Incident, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO incidents(id,title,service_name,raw_logs,firing_rule,environment,status,severity,created_by,created_at,updated_at,resolved_at,source_event_key)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT(source_event_key) WHERE source_event_key IS NOT NULL
		DO UPDATE SET source_event_key=EXCLUDED.source_event_key
		RETURNING `+incidentColumns,
		incident.ID, incident.Title, incident.ServiceName, incident.RawLogs, incident.FiringRule,
		incident.Environment, incident.Status, incident.Severity, incident.CreatedBy,
		incident.CreatedAt, incident.UpdatedAt, incident.ResolvedAt, incident.SourceEventKey)
	return scanIncident(row)
}

func (s *Store) GetIncident(ctx context.Context, id string) (domain.Incident, error) {
	incident, err := scanIncident(s.pool.QueryRow(ctx, `SELECT `+incidentColumns+` FROM incidents WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Incident{}, storecontract.ErrNotFound
	}
	return incident, err
}

func (s *Store) ListIncidents(ctx context.Context, page, size int) (domain.Page[domain.Incident], error) {
	return s.listIncidents(ctx, page, size, `ORDER BY created_at DESC`)
}

func (s *Store) ListActiveIncidents(ctx context.Context) ([]domain.Incident, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+incidentColumns+` FROM incidents WHERE status IN ('OPEN','ANALYZING') ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	incidents := make([]domain.Incident, 0)
	for rows.Next() {
		incident, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, incident)
	}
	return incidents, rows.Err()
}

func (s *Store) SetIncidentStatus(ctx context.Context, id string, status domain.IncidentStatus, updatedAt time.Time) error {
	result, err := s.pool.Exec(ctx, `UPDATE incidents SET status=$2, updated_at=$3 WHERE id=$1`, id, status, updatedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return storecontract.ErrNotFound
	}
	return nil
}

func (s *Store) CompleteTriage(ctx context.Context, analysis domain.IncidentAnalysis, severity domain.IncidentSeverity, updatedAt time.Time) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(tx)
	result, err := tx.Exec(ctx, `UPDATE incidents SET severity=$2,status='ANALYZING',updated_at=$3 WHERE id=$1`, analysis.IncidentID, severity, updatedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return storecontract.ErrNotFound
	}
	components := analysis.AffectedComponents
	if components == nil {
		components = []string{}
	}
	encodedComponents, err := json.Marshal(components)
	if err != nil {
		return err
	}
	citedPaths := analysis.CitedSourcePaths
	if citedPaths == nil {
		citedPaths = []string{}
	}
	encodedCitedPaths, err := json.Marshal(citedPaths)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO incident_analyses(
			id,incident_id,root_cause,immediate_mitigation,confidence_score,severity,
			affected_components,created_at,unified_diff,verification_plan,rollback_plan,
			cited_source_paths,repository_url,commit_sha,commit_message,repository_provider,
			target_branch,ci_provider,build_url)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		analysis.ID, analysis.IncidentID, analysis.RootCause, analysis.ImmediateMitigation,
		analysis.ConfidenceScore, analysis.Severity, string(encodedComponents), analysis.CreatedAt,
		analysis.UnifiedDiff, analysis.VerificationPlan, analysis.RollbackPlan, string(encodedCitedPaths),
		nullableString(analysis.RepositoryURL), nullableString(analysis.CommitSHA), nullableString(analysis.CommitMessage),
		nullableString(analysis.RepositoryProvider), nullableString(analysis.TargetBranch),
		nullableString(analysis.CIProvider), nullableString(analysis.BuildURL))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) LatestAnalysis(ctx context.Context, incidentID string) (*domain.IncidentAnalysis, error) {
	var analysis domain.IncidentAnalysis
	var encodedComponents string
	var encodedCitedPaths string
	err := s.pool.QueryRow(ctx, `
		SELECT id,incident_id,root_cause,COALESCE(immediate_mitigation,''),COALESCE(confidence_score,''),
			COALESCE(severity,''),affected_components,created_at,COALESCE(unified_diff,''),
			COALESCE(verification_plan,''),COALESCE(rollback_plan,''),cited_source_paths,
			COALESCE(repository_url,''),COALESCE(commit_sha,''),COALESCE(commit_message,''),
			COALESCE(repository_provider,''),COALESCE(target_branch,''),COALESCE(ci_provider,''),
			COALESCE(build_url,'')
		FROM incident_analyses WHERE incident_id=$1 ORDER BY created_at DESC LIMIT 1`, incidentID).
		Scan(&analysis.ID, &analysis.IncidentID, &analysis.RootCause, &analysis.ImmediateMitigation,
			&analysis.ConfidenceScore, &analysis.Severity, &encodedComponents, &analysis.CreatedAt,
			&analysis.UnifiedDiff, &analysis.VerificationPlan, &analysis.RollbackPlan, &encodedCitedPaths,
			&analysis.RepositoryURL, &analysis.CommitSHA, &analysis.CommitMessage,
			&analysis.RepositoryProvider, &analysis.TargetBranch, &analysis.CIProvider, &analysis.BuildURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(encodedComponents), &analysis.AffectedComponents); err != nil {
		return nil, fmt.Errorf("decode affected components: %w", err)
	}
	if err := json.Unmarshal([]byte(encodedCitedPaths), &analysis.CitedSourcePaths); err != nil {
		return nil, fmt.Errorf("decode cited source paths: %w", err)
	}
	return &analysis, nil
}

func (s *Store) SaveRemediation(ctx context.Context, remediation domain.Remediation) (domain.Remediation, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO remediation_records(id,incident_id,ai_root_cause,suggested_action,script_type,executable_script,confidence_score,affected_components,requires_approval,applied_by,execution_status,created_at,applied_at,unified_diff,verification_plan,rollback_plan)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING `+remediationColumns,
		remediation.ID, remediation.IncidentID, remediation.AIRootCause, remediation.SuggestedAction,
		remediation.ScriptType, remediation.ExecutableScript, remediation.ConfidenceScore,
		remediation.AffectedParts, remediation.RequiresApproval, remediation.AppliedBy,
		remediation.ExecutionStatus, remediation.CreatedAt, remediation.AppliedAt,
		remediation.UnifiedDiff, remediation.VerificationPlan, remediation.RollbackPlan)
	return scanRemediation(row)
}

func (s *Store) ApproveRemediation(ctx context.Context, incidentID, remediationID, approvedBy string, approvedAt time.Time) (domain.Remediation, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Remediation{}, err
	}
	defer rollback(tx)
	var actualIncidentID string
	err = tx.QueryRow(ctx, `SELECT incident_id FROM remediation_records WHERE id=$1 FOR UPDATE`, remediationID).Scan(&actualIncidentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Remediation{}, storecontract.ErrNotFound
	}
	if err != nil {
		return domain.Remediation{}, err
	}
	if actualIncidentID != incidentID {
		return domain.Remediation{}, storecontract.ErrIncidentMismatch
	}
	remediation, err := scanRemediation(tx.QueryRow(ctx, `
		UPDATE remediation_records SET execution_status='APPROVED',applied_by=$2,applied_at=NULL
		WHERE id=$1 RETURNING `+remediationColumns, remediationID, approvedBy))
	if err != nil {
		return domain.Remediation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Remediation{}, err
	}
	return remediation, nil
}

func (s *Store) DashboardStats(ctx context.Context, start, end time.Time) (domain.DashboardStats, error) {
	var stats domain.DashboardStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM incidents WHERE status='OPEN'),
			(SELECT COUNT(*) FROM incidents WHERE status='ANALYZING'),
			(SELECT COUNT(*) FROM incidents WHERE status='RESOLVED' AND resolved_at BETWEEN $1 AND $2),
			(SELECT COUNT(*) FROM remediation_records WHERE execution_status='PENDING'),
			(SELECT COUNT(*) FROM remediation_records WHERE execution_status='APPLIED')`, start, end).
		Scan(&stats.OpenIncidents, &stats.AnalyzingIncidents, &stats.ResolvedToday,
			&stats.PendingRemediations, &stats.AppliedRemediations)
	return stats, err
}

func (s *Store) listIncidents(ctx context.Context, page, size int, order string) (domain.Page[domain.Incident], error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.Page[domain.Incident]{}, err
	}
	defer rollback(tx)
	var total int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM incidents`).Scan(&total); err != nil {
		return domain.Page[domain.Incident]{}, err
	}
	rows, err := tx.Query(ctx, `SELECT `+incidentColumns+` FROM incidents `+order+` LIMIT $1 OFFSET $2`, size, page*size)
	if err != nil {
		return domain.Page[domain.Incident]{}, err
	}
	defer rows.Close()
	content := make([]domain.Incident, 0, size)
	for rows.Next() {
		incident, err := scanIncident(rows)
		if err != nil {
			return domain.Page[domain.Incident]{}, err
		}
		content = append(content, incident)
	}
	if err := rows.Err(); err != nil {
		return domain.Page[domain.Incident]{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Page[domain.Incident]{}, err
	}
	return domain.Page[domain.Incident]{Content: content, TotalElements: total, Page: page, Size: size}, nil
}

func scanIncident(row rowScanner) (domain.Incident, error) {
	var incident domain.Incident
	var status string
	var severity *string
	err := row.Scan(&incident.ID, &incident.Title, &incident.ServiceName, &incident.RawLogs,
		&incident.FiringRule, &incident.Environment, &status, &severity, &incident.CreatedBy,
		&incident.CreatedAt, &incident.UpdatedAt, &incident.ResolvedAt, &incident.SourceEventKey)
	if err != nil {
		return domain.Incident{}, err
	}
	incident.Status = domain.IncidentStatus(status)
	if severity != nil {
		value := domain.IncidentSeverity(*severity)
		incident.Severity = &value
	}
	return incident, nil
}

func scanRemediation(row rowScanner) (domain.Remediation, error) {
	var remediation domain.Remediation
	var executionStatus string
	err := row.Scan(&remediation.ID, &remediation.IncidentID, &remediation.AIRootCause,
		&remediation.SuggestedAction, &remediation.ScriptType, &remediation.ExecutableScript,
		&remediation.ConfidenceScore, &remediation.AffectedParts, &remediation.RequiresApproval,
		&remediation.AppliedBy, &executionStatus, &remediation.CreatedAt, &remediation.AppliedAt,
		&remediation.UnifiedDiff, &remediation.VerificationPlan, &remediation.RollbackPlan)
	if err != nil {
		return domain.Remediation{}, err
	}
	remediation.ExecutionStatus = domain.ExecutionStatus(executionStatus)
	return remediation, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
