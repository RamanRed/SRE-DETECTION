package postgres

import (
	"context"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

const pipelineColumns = `id,pipeline_name,build_number,ci_tool,status,git_commit,git_branch,commit_message,author,duration_seconds,tests_passed,tests_failed,vulnerabilities_detected,environment,log_snippet,build_url,created_at,updated_at,external_build_id`

func (s *Store) CreatePipelineBuild(ctx context.Context, build domain.PipelineBuild) (domain.PipelineBuild, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO pipeline_builds(id,pipeline_name,build_number,ci_tool,status,git_commit,git_branch,commit_message,author,duration_seconds,tests_passed,tests_failed,vulnerabilities_detected,environment,log_snippet,build_url,created_at,updated_at,external_build_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		ON CONFLICT(ci_tool,pipeline_name,external_build_id) WHERE external_build_id IS NOT NULL
		DO UPDATE SET status=EXCLUDED.status,duration_seconds=EXCLUDED.duration_seconds,
			log_snippet=EXCLUDED.log_snippet,build_url=EXCLUDED.build_url,updated_at=EXCLUDED.updated_at
		RETURNING `+pipelineColumns, pipelineArgs(build)...)
	return scanPipelineBuild(row)
}

func (s *Store) ListPipelineBuilds(ctx context.Context, page, size int) (domain.Page[domain.PipelineBuild], error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.Page[domain.PipelineBuild]{}, err
	}
	defer rollback(tx)
	var total int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM pipeline_builds`).Scan(&total); err != nil {
		return domain.Page[domain.PipelineBuild]{}, err
	}
	rows, err := tx.Query(ctx, `SELECT `+pipelineColumns+` FROM pipeline_builds ORDER BY created_at DESC LIMIT $1 OFFSET $2`, size, page*size)
	if err != nil {
		return domain.Page[domain.PipelineBuild]{}, err
	}
	defer rows.Close()
	content := make([]domain.PipelineBuild, 0, size)
	for rows.Next() {
		build, err := scanPipelineBuild(rows)
		if err != nil {
			return domain.Page[domain.PipelineBuild]{}, err
		}
		content = append(content, build)
	}
	if err := rows.Err(); err != nil {
		return domain.Page[domain.PipelineBuild]{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Page[domain.PipelineBuild]{}, err
	}
	return domain.Page[domain.PipelineBuild]{Content: content, TotalElements: total, Page: page, Size: size}, nil
}

func (s *Store) DORAMetrics(ctx context.Context, after time.Time) (domain.DORARawMetrics, error) {
	var metrics domain.DORARawMetrics
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status='SUCCESS' AND created_at >= $1),
			COUNT(*) FILTER (WHERE status IN ('FAILURE','UNSTABLE','CANCELLED','TIMED_OUT') AND created_at >= $1),
			AVG(duration_seconds) FILTER (WHERE created_at >= $1)
		FROM pipeline_builds`, after).
		Scan(&metrics.TotalBuilds, &metrics.SuccessfulBuilds, &metrics.FailedBuilds, &metrics.AverageDuration)
	return metrics, err
}

func pipelineArgs(build domain.PipelineBuild) []any {
	return []any{
		build.ID, build.PipelineName, build.BuildNumber, build.CITool, build.Status,
		build.GitCommit, build.GitBranch, build.CommitMessage, build.Author,
		build.DurationSeconds, build.TestsPassed, build.TestsFailed,
		build.VulnerabilitiesDetected, build.Environment, build.LogSnippet,
		build.BuildURL, build.CreatedAt, build.UpdatedAt,
		build.ExternalBuildID,
	}
}

func scanPipelineBuild(row rowScanner) (domain.PipelineBuild, error) {
	var build domain.PipelineBuild
	err := row.Scan(&build.ID, &build.PipelineName, &build.BuildNumber, &build.CITool,
		&build.Status, &build.GitCommit, &build.GitBranch, &build.CommitMessage,
		&build.Author, &build.DurationSeconds, &build.TestsPassed, &build.TestsFailed,
		&build.VulnerabilitiesDetected, &build.Environment, &build.LogSnippet,
		&build.BuildURL, &build.CreatedAt, &build.UpdatedAt, &build.ExternalBuildID)
	return build, err
}
