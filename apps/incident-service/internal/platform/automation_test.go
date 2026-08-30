package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

func TestGitLabLatestCommitUsesEscapedProjectAndPrivateToken(t *testing.T) {
	var captured *http.Request
	client, err := NewWithHTTPClient("https://api.github.test", roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		captured = request
		body := `{"id":"abc123","message":"fix pool","author_name":"Dev","web_url":"https://gitlab.com/group/project/-/commit/abc123","committed_date":"2026-08-30T10:00:00Z"}`
		return response(http.StatusOK, body), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	provider, repositoryURL, branch, token := "GITLAB", "https://gitlab.com/group/subgroup/project", "main", "secret"
	commit, err := client.LatestCommit(t.Context(), domain.PlatformIntegration{
		RepositoryProvider: &provider, RepositoryURL: &repositoryURL, TargetBranch: &branch, RepositoryToken: &token,
	})
	if err != nil {
		t.Fatal(err)
	}
	if commit.SHA != "abc123" || captured.Header.Get("PRIVATE-TOKEN") != token {
		t.Fatalf("commit=%+v headers=%v", commit, captured.Header)
	}
	if !strings.Contains(captured.URL.EscapedPath(), "group%2Fsubgroup%2Fproject") {
		t.Fatalf("GitLab project was not one escaped path segment: %s", captured.URL.EscapedPath())
	}
}

func TestGitHubActionsDispatchDoesNotAttachOlderRun(t *testing.T) {
	requests := make([]*http.Request, 0, 2)
	client, err := NewWithHTTPClient("https://api.github.test", roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request)
		if request.Method == http.MethodPost {
			return response(http.StatusNoContent, ""), nil
		}
		return response(http.StatusOK, `{"workflow_runs":[{"id":42,"run_number":9,"status":"completed","conclusion":"success","head_sha":"older"}]}`), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	provider, repositoryURL, branch := "GITHUB", "https://github.com/acme/widgets", "main"
	engine, job, ciBase := "GITHUB_ACTIONS", "ci.yml", "https://api.github.com"
	commit := domain.CommitMetadata{SHA: "new-sha", Branch: branch}
	build, err := client.TriggerBuild(t.Context(), domain.PlatformIntegration{
		RepositoryProvider: &provider, RepositoryURL: &repositoryURL, TargetBranch: &branch,
		PipelineEngine: &engine, JobName: &job, CIBaseURL: &ciBase,
	}, commit)
	if err != nil {
		t.Fatal(err)
	}
	if build.ID != "" || build.Status != "QUEUED" || build.Reference != commit.SHA {
		t.Fatalf("dispatch attached an older run: %+v", build)
	}
	if len(requests) != 2 || requests[1].URL.Query().Get("head_sha") != commit.SHA {
		t.Fatalf("workflow query = %v", requests)
	}
}

func TestKubernetesJobCloneRemovesControllerIdentity(t *testing.T) {
	var posted map[string]any
	call := 0
	client, err := NewWithHTTPClient("https://api.github.test", roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		call++
		switch call {
		case 1:
			fixture := `{
				"metadata":{"labels":{"team":"sre","controller-uid":"old","batch.kubernetes.io/job-name":"old"}},
				"spec":{"manualSelector":true,"selector":{"matchLabels":{"controller-uid":"old"}},
				"template":{"metadata":{"labels":{"app":"worker","job-name":"old","batch.kubernetes.io/controller-uid":"old"}},"spec":{"restartPolicy":"Never","containers":[{"name":"worker","image":"example/worker"}]}}}}
			`
			return response(http.StatusOK, fixture), nil
		case 2:
			if !strings.Contains(request.URL.Path, "/namespaces/sre-copilot/jobs") {
				t.Fatalf("unexpected Kubernetes path: %s", request.URL.Path)
			}
			if err := json.NewDecoder(request.Body).Decode(&posted); err != nil {
				t.Fatal(err)
			}
			return response(http.StatusCreated, `{"metadata":{"name":"rebuild-abcdef12-x1"}}`), nil
		default:
			t.Fatalf("unexpected request %d: %s", call, request.URL)
			return nil, errors.New("unexpected request")
		}
	}), true)
	if err != nil {
		t.Fatal(err)
	}
	engine, baseURL, job, token := "KUBERNETES_JOB", "https://kubernetes.default.svc", "rebuild", "service-token"
	build, err := client.TriggerBuild(t.Context(), domain.PlatformIntegration{
		PipelineEngine: &engine, CIBaseURL: &baseURL, JobName: &job, CIToken: &token,
	}, domain.CommitMetadata{SHA: "ABCDEF123456"})
	if err != nil {
		t.Fatal(err)
	}
	if build.ID != "rebuild-abcdef12-x1" {
		t.Fatalf("build = %+v", build)
	}
	metadata := posted["metadata"].(map[string]any)
	if metadata["generateName"] != "rebuild-abcdef12-" {
		t.Fatalf("generateName = %v", metadata["generateName"])
	}
	labels := metadata["labels"].(map[string]any)
	if labels["team"] != "sre" || labels["controller-uid"] != nil || labels["batch.kubernetes.io/job-name"] != nil {
		t.Fatalf("metadata labels were not sanitized: %#v", labels)
	}
	spec := posted["spec"].(map[string]any)
	if spec["selector"] != nil || spec["manualSelector"] != nil {
		t.Fatalf("server-generated selector survived: %#v", spec)
	}
	template := spec["template"].(map[string]any)
	templateLabels := template["metadata"].(map[string]any)["labels"].(map[string]any)
	if templateLabels["app"] != "worker" || templateLabels["job-name"] != nil || templateLabels["batch.kubernetes.io/controller-uid"] != nil {
		t.Fatalf("pod labels were not sanitized: %#v", templateLabels)
	}
}

func TestFailedKubernetesJobFetchesBoundedPodLogs(t *testing.T) {
	call := 0
	client, err := NewWithHTTPClient("https://api.github.test", roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		call++
		switch call {
		case 1:
			return response(http.StatusOK, `{"status":{"failed":1,"startTime":"2026-08-30T10:00:00Z","completionTime":"2026-08-30T10:01:00Z","conditions":[{"type":"Failed","status":"True"}]}}`), nil
		case 2:
			return response(http.StatusOK, `{"items":[{"metadata":{"name":"failed-pod"}}]}`), nil
		case 3:
			if request.URL.Query().Get("limitBytes") == "" || !strings.HasSuffix(request.URL.Path, "/failed-pod/log") {
				t.Fatalf("unexpected log request: %s", request.URL)
			}
			return response(http.StatusOK, "src/main.go:42: connection refused"), nil
		default:
			return nil, errors.New("unexpected request")
		}
	}), true)
	if err != nil {
		t.Fatal(err)
	}
	engine, baseURL := "KUBERNETES_JOB", "https://kubernetes.default.svc"
	build, err := client.PollBuild(t.Context(), domain.PlatformIntegration{PipelineEngine: &engine, CIBaseURL: &baseURL}, domain.CIBuild{ID: "failed-job"})
	if err != nil {
		t.Fatal(err)
	}
	if build.Status != "FAILURE" || !strings.Contains(build.Logs, "src/main.go:42") {
		t.Fatalf("build = %+v", build)
	}
}

func TestKubernetesJobFailedPodAttemptIsNotTerminalWithoutCondition(t *testing.T) {
	client, err := NewWithHTTPClient("https://api.github.test", roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"status":{"active":1,"failed":1,"startTime":"2026-08-30T10:00:00Z","conditions":[]}}`), nil
	}), true)
	if err != nil {
		t.Fatal(err)
	}
	engine, baseURL := "KUBERNETES_JOB", "https://kubernetes.default.svc"
	build, err := client.PollBuild(t.Context(), domain.PlatformIntegration{PipelineEngine: &engine, CIBaseURL: &baseURL}, domain.CIBuild{ID: "retrying-job"})
	if err != nil {
		t.Fatal(err)
	}
	if build.Status != "RUNNING" {
		t.Fatalf("retrying Job status = %q, want RUNNING", build.Status)
	}
}

func TestPrivateModeStillRejectsDNSResolvedLoopbackAndLinkLocal(t *testing.T) {
	dial := guardedDialer(50*time.Millisecond, true)
	if _, err := dial(context.Background(), "tcp", "localhost:80"); err == nil {
		t.Fatal("private mode accepted a hostname resolving to loopback")
	}
	if _, err := safeIntegrationURL("http://169.254.169.254/latest/meta-data", true); err == nil {
		t.Fatal("private mode accepted a link-local metadata endpoint")
	}
}

func TestKubernetesRequestReloadsRotatingServiceAccountToken(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("first-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	var authorizations []string
	client, err := NewWithHTTPClient("https://api.github.test", roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		authorizations = append(authorizations, request.Header.Get("Authorization"))
		return response(http.StatusOK, `{}`), nil
	}), true)
	if err != nil {
		t.Fatal(err)
	}
	client.kubernetesTokenFile = tokenFile
	engine, baseURL, job := "KUBERNETES_JOB", "https://kubernetes.default.svc", "rebuild"
	integration := domain.PlatformIntegration{PipelineEngine: &engine, CIBaseURL: &baseURL, JobName: &job}
	if err := client.TestCI(t.Context(), integration); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenFile, []byte("second-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := client.TestCI(t.Context(), integration); err != nil {
		t.Fatal(err)
	}
	if len(authorizations) != 2 || authorizations[0] != "Bearer first-token" || authorizations[1] != "Bearer second-token" {
		t.Fatalf("authorization headers = %#v", authorizations)
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
