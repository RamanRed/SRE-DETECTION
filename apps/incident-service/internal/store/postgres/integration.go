package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

const integrationColumns = `id,user_id,username,github_token,github_repo,github_branch,github_status,jenkins_url,jenkins_username,jenkins_api_token,jenkins_job_name,jenkins_status,last_sync_time,created_at,updated_at,repository_provider,repository_url,target_branch,pipeline_engine,ci_base_url,ci_username,job_name,polling_cadence,auto_rebuild,auto_ai_triage,connection_status,last_polled_commit,last_poll_time,next_poll_time`

func (s *Store) GetIntegration(ctx context.Context, userID string) (*domain.PlatformIntegration, error) {
	integration, err := scanIntegration(s.pool.QueryRow(ctx, `SELECT `+integrationColumns+` FROM platform_integrations WHERE user_id=$1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.decryptIntegration(&integration); err != nil {
		return nil, err
	}
	return &integration, nil
}

func (s *Store) SaveIntegration(ctx context.Context, integration domain.PlatformIntegration, now time.Time) (domain.PlatformIntegration, error) {
	createdAt := now
	if integration.CreatedAt != nil {
		createdAt = *integration.CreatedAt
	}
	repositoryToken := integration.RepositoryToken
	if repositoryToken == nil {
		repositoryToken = integration.GitHubToken
	}
	ciToken := integration.CIToken
	if ciToken == nil {
		ciToken = integration.JenkinsAPIToken
	}
	encryptedRepositoryToken, err := s.encryptOptional(repositoryToken)
	if err != nil {
		return domain.PlatformIntegration{}, err
	}
	encryptedCIToken, err := s.encryptOptional(ciToken)
	if err != nil {
		return domain.PlatformIntegration{}, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO platform_integrations(
			user_id,username,github_token,github_repo,github_branch,github_status,
			jenkins_url,jenkins_username,jenkins_api_token,jenkins_job_name,jenkins_status,
			last_sync_time,created_at,updated_at,repository_provider,repository_url,target_branch,
			pipeline_engine,ci_base_url,ci_username,job_name,polling_cadence,auto_rebuild,
			auto_ai_triage,connection_status,last_polled_commit,last_poll_time,next_poll_time)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28)
		ON CONFLICT(user_id) DO UPDATE SET
			username=EXCLUDED.username,
			github_token=EXCLUDED.github_token,
			github_repo=EXCLUDED.github_repo,
			github_branch=EXCLUDED.github_branch,
			github_status=EXCLUDED.github_status,
			jenkins_url=EXCLUDED.jenkins_url,
			jenkins_username=EXCLUDED.jenkins_username,
			jenkins_api_token=EXCLUDED.jenkins_api_token,
			jenkins_job_name=EXCLUDED.jenkins_job_name,
			jenkins_status=EXCLUDED.jenkins_status,
			last_sync_time=EXCLUDED.last_sync_time,
			repository_provider=EXCLUDED.repository_provider,
			repository_url=EXCLUDED.repository_url,
			target_branch=EXCLUDED.target_branch,
			pipeline_engine=EXCLUDED.pipeline_engine,
			ci_base_url=EXCLUDED.ci_base_url,
			ci_username=EXCLUDED.ci_username,
			job_name=EXCLUDED.job_name,
			polling_cadence=EXCLUDED.polling_cadence,
			auto_rebuild=EXCLUDED.auto_rebuild,
			auto_ai_triage=EXCLUDED.auto_ai_triage,
			connection_status=EXCLUDED.connection_status,
			last_polled_commit=EXCLUDED.last_polled_commit,
			last_poll_time=EXCLUDED.last_poll_time,
			next_poll_time=EXCLUDED.next_poll_time,
			updated_at=EXCLUDED.updated_at
		RETURNING `+integrationColumns,
		integration.UserID, integration.Username, encryptedRepositoryToken, integration.GitHubRepo,
		integration.GitHubBranch, integration.GitHubStatus, integration.JenkinsURL,
		integration.JenkinsUsername, encryptedCIToken, integration.JenkinsJobName,
		integration.JenkinsStatus, integration.LastSyncTime, createdAt, now,
		integration.RepositoryProvider, integration.RepositoryURL, integration.TargetBranch,
		integration.PipelineEngine, integration.CIBaseURL, integration.CIUsername,
		integration.JobName, integration.PollingCadence, integration.AutoRebuild,
		integration.AutoAITriage, integration.ConnectionStatus, integration.LastPolledCommit,
		integration.LastPollTime, integration.NextPollTime)
	saved, err := scanIntegration(row)
	if err != nil {
		return domain.PlatformIntegration{}, err
	}
	if err := s.decryptIntegration(&saved); err != nil {
		return domain.PlatformIntegration{}, err
	}
	return saved, nil
}

func (s *Store) ClaimDueIntegrations(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.PlatformIntegration, error) {
	if limit < 1 {
		return []domain.PlatformIntegration{}, nil
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	rows, err := tx.Query(ctx, `
		SELECT `+integrationColumns+`
		FROM platform_integrations
		WHERE connection_status IN ('CONNECTED','ERROR','SYNCING')
		  AND next_poll_time IS NOT NULL AND next_poll_time <= $1
		ORDER BY next_poll_time,id
		FOR UPDATE SKIP LOCKED
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	integrations := make([]domain.PlatformIntegration, 0, limit)
	for rows.Next() {
		integration, scanErr := scanIntegration(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		integrations = append(integrations, integration)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range integrations {
		if integrations[index].ID == nil {
			return nil, errors.New("claimed integration has no ID")
		}
		_, err = tx.Exec(ctx, `
			UPDATE platform_integrations
			SET connection_status='SYNCING',last_poll_time=$2,next_poll_time=$3,updated_at=$2
			WHERE id=$1`, *integrations[index].ID, now, now.Add(lease))
		if err != nil {
			return nil, err
		}
		status := "SYNCING"
		leaseUntil := now.Add(lease)
		integrations[index].ConnectionStatus = &status
		integrations[index].LastPollTime = &now
		integrations[index].NextPollTime = &leaseUntil
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for index := range integrations {
		if err := s.decryptIntegration(&integrations[index]); err != nil {
			return nil, err
		}
	}
	return integrations, nil
}

func (s *Store) RecordCommitFingerprint(ctx context.Context, integrationID int64, fingerprint string, commit domain.CommitMetadata, discoveredAt time.Time) (bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	result, err := tx.Exec(ctx, `
		INSERT INTO integration_commit_fingerprints(integration_id,fingerprint,commit_sha,commit_message,discovered_at,state,processed_at,updated_at)
		VALUES($1,$2,$3,$4,$5,'PROCESSED',$5,$5)
		ON CONFLICT(integration_id,fingerprint) DO UPDATE
		SET state='PROCESSED',processed_at=EXCLUDED.processed_at,updated_at=EXCLUDED.updated_at,last_error=NULL`,
		integrationID, fingerprint, commit.SHA, commit.Message, discoveredAt)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

func (s *Store) BeginCommitProcessing(ctx context.Context, integrationID int64, fingerprint string, commit domain.CommitMetadata, now time.Time, staleAfter time.Duration) (domain.CommitProcessing, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.CommitProcessing{}, err
	}
	defer rollback(tx)
	var state string
	var buildState *string
	var updatedAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT state,build_state,updated_at
		FROM integration_commit_fingerprints
		WHERE integration_id=$1 AND fingerprint=$2
		FOR UPDATE`, integrationID, fingerprint).Scan(&state, &buildState, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `
			INSERT INTO integration_commit_fingerprints(
				integration_id,fingerprint,commit_sha,commit_message,discovered_at,state,attempt_count,updated_at)
			VALUES($1,$2,$3,$4,$5,'PROCESSING',1,$5)`,
			integrationID, fingerprint, commit.SHA, commit.Message, now)
		if err != nil {
			return domain.CommitProcessing{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.CommitProcessing{}, err
		}
		return domain.CommitProcessing{ShouldProcess: true}, nil
	}
	if err != nil {
		return domain.CommitProcessing{}, err
	}
	if state == "PROCESSED" || (state == "PROCESSING" && now.Sub(updatedAt) < staleAfter) {
		if err := tx.Commit(ctx); err != nil {
			return domain.CommitProcessing{}, err
		}
		return domain.CommitProcessing{}, nil
	}
	_, err = tx.Exec(ctx, `
		UPDATE integration_commit_fingerprints
		SET state='PROCESSING',attempt_count=attempt_count+1,last_error=NULL,updated_at=$3
		WHERE integration_id=$1 AND fingerprint=$2`, integrationID, fingerprint, now)
	if err != nil {
		return domain.CommitProcessing{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.CommitProcessing{}, err
	}
	result := domain.CommitProcessing{ShouldProcess: true}
	if buildState != nil && *buildState != "" {
		var build domain.CIBuild
		if err := json.Unmarshal([]byte(*buildState), &build); err != nil {
			return domain.CommitProcessing{}, err
		}
		result.Build = &build
	}
	return result, nil
}

func (s *Store) SaveCommitBuild(ctx context.Context, integrationID int64, fingerprint string, build domain.CIBuild, now time.Time) error {
	encoded, err := json.Marshal(build)
	if err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `
		UPDATE integration_commit_fingerprints SET build_state=$3,updated_at=$4
		WHERE integration_id=$1 AND fingerprint=$2 AND state='PROCESSING'`,
		integrationID, fingerprint, string(encoded), now)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) FinishCommitProcessing(ctx context.Context, integrationID int64, fingerprint string, success bool, failure string, now time.Time) error {
	state := "FAILED"
	var processedAt *time.Time
	if success {
		state = "PROCESSED"
		processedAt = &now
		failure = ""
	}
	if len(failure) > 2048 {
		failure = failure[:2048]
	}
	result, err := s.pool.Exec(ctx, `
		UPDATE integration_commit_fingerprints
		SET state=$3,last_error=NULLIF($4,''),updated_at=$5,processed_at=$6
		WHERE integration_id=$1 AND fingerprint=$2`,
		integrationID, fingerprint, state, failure, now, processedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) CompleteIntegrationPoll(ctx context.Context, userID, status string, lastCommit *string, completedAt, nextPoll time.Time) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE platform_integrations
		SET connection_status=$2,last_polled_commit=COALESCE($3,last_polled_commit),
			last_sync_time=$4,last_poll_time=$4,next_poll_time=$5,updated_at=$4,
			github_status=$2,jenkins_status=$2
		WHERE user_id=$1`, userID, status, lastCommit, completedAt, nextPoll)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func scanIntegration(row rowScanner) (domain.PlatformIntegration, error) {
	var integration domain.PlatformIntegration
	var id int64
	err := row.Scan(&id, &integration.UserID, &integration.Username, &integration.GitHubToken,
		&integration.GitHubRepo, &integration.GitHubBranch, &integration.GitHubStatus,
		&integration.JenkinsURL, &integration.JenkinsUsername, &integration.JenkinsAPIToken,
		&integration.JenkinsJobName, &integration.JenkinsStatus, &integration.LastSyncTime,
		&integration.CreatedAt, &integration.UpdatedAt, &integration.RepositoryProvider,
		&integration.RepositoryURL, &integration.TargetBranch, &integration.PipelineEngine,
		&integration.CIBaseURL, &integration.CIUsername, &integration.JobName,
		&integration.PollingCadence, &integration.AutoRebuild, &integration.AutoAITriage,
		&integration.ConnectionStatus, &integration.LastPolledCommit, &integration.LastPollTime,
		&integration.NextPollTime)
	if err != nil {
		return domain.PlatformIntegration{}, err
	}
	integration.ID = &id
	integration.RepositoryToken = integration.GitHubToken
	integration.CIToken = integration.JenkinsAPIToken
	return integration, nil
}

func (s *Store) encryptOptional(value *string) (*string, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	encrypted, err := s.secrets.Encrypt(*value)
	if err != nil {
		return nil, err
	}
	return &encrypted, nil
}

func (s *Store) decryptIntegration(integration *domain.PlatformIntegration) error {
	decrypt := func(value **string) error {
		if *value == nil || **value == "" {
			return nil
		}
		plaintext, err := s.secrets.Decrypt(**value)
		if err != nil {
			return err
		}
		*value = &plaintext
		return nil
	}
	if err := decrypt(&integration.GitHubToken); err != nil {
		return err
	}
	if err := decrypt(&integration.JenkinsAPIToken); err != nil {
		return err
	}
	integration.RepositoryToken = integration.GitHubToken
	integration.CIToken = integration.JenkinsAPIToken
	return nil
}
