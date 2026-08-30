package platform

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

func (c *Client) TestCI(ctx context.Context, integration domain.PlatformIntegration) error {
	engine := strings.ToUpper(integrationString(integration.PipelineEngine, nil, "JENKINS"))
	switch engine {
	case "JENKINS":
		request, err := c.jenkinsAutomationRequest(ctx, integration, http.MethodGet, "api/json", nil)
		if err != nil {
			return err
		}
		return c.doStatus(request)
	case "GITHUB_ACTIONS":
		request, err := c.githubActionsRequest(ctx, integration, http.MethodGet, "workflow", nil)
		if err != nil {
			return err
		}
		return c.doStatus(request)
	case "KUBERNETES_JOB":
		request, err := c.kubernetesRequest(ctx, integration, http.MethodGet, c.kubernetesJobPath(integration, ""), nil)
		if err != nil {
			return err
		}
		return c.doStatus(request)
	default:
		return errors.New("unsupported CI engine")
	}
}

func (c *Client) TriggerBuild(ctx context.Context, integration domain.PlatformIntegration, commit domain.CommitMetadata) (domain.CIBuild, error) {
	engine := strings.ToUpper(integrationString(integration.PipelineEngine, nil, "JENKINS"))
	switch engine {
	case "JENKINS":
		return c.triggerJenkins(ctx, integration, commit)
	case "GITHUB_ACTIONS":
		return c.triggerGitHubActions(ctx, integration, commit)
	case "KUBERNETES_JOB":
		return c.triggerKubernetesJob(ctx, integration, commit)
	default:
		return domain.CIBuild{}, errors.New("unsupported CI engine")
	}
}

func (c *Client) LatestBuild(ctx context.Context, integration domain.PlatformIntegration, commit domain.CommitMetadata) (domain.CIBuild, error) {
	engine := strings.ToUpper(integrationString(integration.PipelineEngine, nil, "JENKINS"))
	switch engine {
	case "JENKINS":
		request, err := c.jenkinsAutomationRequest(ctx, integration, http.MethodGet, "lastBuild/api/json", nil)
		if err != nil {
			return domain.CIBuild{}, err
		}
		return c.readJenkinsBuild(request, integration)
	case "GITHUB_ACTIONS":
		return c.latestGitHubActionsRun(ctx, integration, commit)
	case "KUBERNETES_JOB":
		return domain.CIBuild{}, errors.New("Kubernetes Job builds must be triggered before polling")
	default:
		return domain.CIBuild{}, errors.New("unsupported CI engine")
	}
}

func (c *Client) PollBuild(ctx context.Context, integration domain.PlatformIntegration, build domain.CIBuild) (domain.CIBuild, error) {
	engine := strings.ToUpper(integrationString(integration.PipelineEngine, nil, build.Provider))
	switch engine {
	case "JENKINS":
		return c.pollJenkins(ctx, integration, build)
	case "GITHUB_ACTIONS":
		return c.pollGitHubActions(ctx, integration, build)
	case "KUBERNETES_JOB":
		return c.pollKubernetesJob(ctx, integration, build)
	default:
		return domain.CIBuild{}, errors.New("unsupported CI engine")
	}
}

func (c *Client) triggerJenkins(ctx context.Context, integration domain.PlatformIntegration, commit domain.CommitMetadata) (domain.CIBuild, error) {
	query := url.Values{}
	query.Set("GIT_COMMIT", commit.SHA)
	query.Set("GIT_BRANCH", commit.Branch)
	request, err := c.jenkinsAutomationRequest(ctx, integration, http.MethodPost, "buildWithParameters?"+query.Encode(), nil)
	if err != nil {
		return domain.CIBuild{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return domain.CIBuild{}, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return domain.CIBuild{}, fmt.Errorf("Jenkins trigger returned HTTP %d", response.StatusCode)
	}
	reference := response.Header.Get("Location")
	if reference == "" {
		reference = strings.TrimSuffix(request.URL.String(), "/buildWithParameters?"+query.Encode()) + "/lastBuild/api/json"
	} else {
		location, parseErr := url.Parse(reference)
		if parseErr != nil {
			return domain.CIBuild{}, errors.New("Jenkins returned an invalid queue Location")
		}
		reference = request.URL.ResolveReference(location).String()
	}
	return domain.CIBuild{Provider: "JENKINS", Reference: reference, Status: "QUEUED"}, nil
}

func (c *Client) pollJenkins(ctx context.Context, integration domain.PlatformIntegration, build domain.CIBuild) (domain.CIBuild, error) {
	if strings.TrimSpace(build.Reference) == "" {
		return c.LatestBuild(ctx, integration, domain.CommitMetadata{})
	}
	reference, err := c.validatedCIReference(integration, build.Reference)
	if err != nil {
		return domain.CIBuild{}, err
	}
	if strings.Contains(reference.Path, "/queue/item/") {
		if !strings.HasSuffix(reference.Path, "/api/json") {
			reference.Path = strings.TrimRight(reference.Path, "/") + "/api/json"
		}
		request, requestErr := c.authenticatedCIRequest(ctx, integration, http.MethodGet, reference.String(), nil)
		if requestErr != nil {
			return domain.CIBuild{}, requestErr
		}
		var queue struct {
			Cancelled  bool `json:"cancelled"`
			Executable *struct {
				Number int    `json:"number"`
				URL    string `json:"url"`
			} `json:"executable"`
		}
		if err := c.doJSON(request, &queue, maxPlatformResponse); err != nil {
			return domain.CIBuild{}, err
		}
		if queue.Cancelled {
			build.Status = "CANCELLED"
			return build, nil
		}
		if queue.Executable == nil {
			build.Status = "QUEUED"
			return build, nil
		}
		build.Number = queue.Executable.Number
		build.URL = queue.Executable.URL
		build.Reference = strings.TrimRight(queue.Executable.URL, "/") + "/api/json"
	}
	reference, err = c.validatedCIReference(integration, build.Reference)
	if err != nil {
		return domain.CIBuild{}, err
	}
	request, err := c.authenticatedCIRequest(ctx, integration, http.MethodGet, reference.String(), nil)
	if err != nil {
		return domain.CIBuild{}, err
	}
	result, err := c.readJenkinsBuild(request, integration)
	if err != nil {
		return domain.CIBuild{}, err
	}
	if result.Number == 0 {
		result.Number = build.Number
	}
	return result, nil
}

func (c *Client) readJenkinsBuild(request *http.Request, integration domain.PlatformIntegration) (domain.CIBuild, error) {
	var response struct {
		Building  bool   `json:"building"`
		Result    string `json:"result"`
		Number    int    `json:"number"`
		URL       string `json:"url"`
		Duration  int64  `json:"duration"`
		Timestamp int64  `json:"timestamp"`
	}
	if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
		return domain.CIBuild{}, err
	}
	status := strings.ToUpper(response.Result)
	if response.Building {
		status = "RUNNING"
	} else if status == "" {
		status = "QUEUED"
	}
	result := domain.CIBuild{
		Provider: "JENKINS", Number: response.Number, Status: status,
		URL: response.URL, Reference: request.URL.String(), DurationSeconds: int(response.Duration / 1000),
	}
	if response.Timestamp > 0 {
		started := time.UnixMilli(response.Timestamp)
		result.StartedAt = &started
	}
	if result.Terminal() && result.StartedAt != nil {
		completed := result.StartedAt.Add(time.Duration(response.Duration) * time.Millisecond)
		result.CompletedAt = &completed
	}
	if result.Failed() {
		consoleURL := strings.TrimSuffix(request.URL.String(), "/api/json") + "/consoleText"
		consoleRequest, err := c.authenticatedCIRequest(request.Context(), integration, http.MethodGet, consoleURL, nil)
		if err == nil {
			if logs, logErr := c.doBytes(consoleRequest, maxSourceResponse); logErr == nil {
				result.Logs = string(logs)
			}
		}
	}
	return result, nil
}

func (c *Client) triggerGitHubActions(ctx context.Context, integration domain.PlatformIntegration, commit domain.CommitMetadata) (domain.CIBuild, error) {
	body, _ := json.Marshal(map[string]string{"ref": integrationString(integration.TargetBranch, integration.GitHubBranch, commit.Branch)})
	request, err := c.githubActionsRequest(ctx, integration, http.MethodPost, "dispatch", bytes.NewReader(body))
	if err != nil {
		return domain.CIBuild{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return domain.CIBuild{}, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.CIBuild{}, fmt.Errorf("GitHub Actions dispatch returned HTTP %d", response.StatusCode)
	}
	return c.latestGitHubActionsRun(ctx, integration, commit)
}

func (c *Client) latestGitHubActionsRun(ctx context.Context, integration domain.PlatformIntegration, commit domain.CommitMetadata) (domain.CIBuild, error) {
	request, err := c.githubActionsRequest(ctx, integration, http.MethodGet, "runs", nil)
	if err != nil {
		return domain.CIBuild{}, err
	}
	query := request.URL.Query()
	query.Set("branch", integrationString(integration.TargetBranch, integration.GitHubBranch, commit.Branch))
	query.Set("per_page", "1")
	if commit.SHA != "" {
		query.Set("head_sha", commit.SHA)
	}
	request.URL.RawQuery = query.Encode()
	var response struct {
		Runs []githubActionsRun `json:"workflow_runs"`
	}
	if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
		return domain.CIBuild{}, err
	}
	if len(response.Runs) == 0 || (commit.SHA != "" && !strings.EqualFold(response.Runs[0].HeadSHA, commit.SHA)) {
		return domain.CIBuild{Provider: "GITHUB_ACTIONS", Reference: commit.SHA, Status: "QUEUED"}, nil
	}
	return mapGitHubActionsRun(response.Runs[0]), nil
}

func (c *Client) pollGitHubActions(ctx context.Context, integration domain.PlatformIntegration, build domain.CIBuild) (domain.CIBuild, error) {
	if build.ID == "" {
		return c.latestGitHubActionsRun(ctx, integration, domain.CommitMetadata{SHA: build.Reference})
	}
	request, err := c.githubActionsRequest(ctx, integration, http.MethodGet, "run/"+url.PathEscape(build.ID), nil)
	if err != nil {
		return domain.CIBuild{}, err
	}
	var response githubActionsRun
	if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
		return domain.CIBuild{}, err
	}
	result := mapGitHubActionsRun(response)
	if result.Failed() {
		if logs, logErr := c.githubActionsLogs(ctx, integration, build.ID); logErr == nil {
			result.Logs = logs
		}
	}
	return result, nil
}

func (c *Client) githubActionsLogs(ctx context.Context, integration domain.PlatformIntegration, runID string) (string, error) {
	request, err := c.githubActionsRequest(ctx, integration, http.MethodGet, "run/"+url.PathEscape(runID)+"/logs", nil)
	if err != nil {
		return "", err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	if response.StatusCode == http.StatusFound || response.StatusCode == http.StatusTemporaryRedirect || response.StatusCode == http.StatusPermanentRedirect {
		location := response.Header.Get("Location")
		response.Body.Close()
		parsed, parseErr := safeIntegrationURL(location, false)
		if parseErr != nil || parsed.Scheme != "https" {
			return "", errors.New("GitHub Actions logs redirect was unsafe")
		}
		redirectRequest, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if requestErr != nil {
			return "", requestErr
		}
		// Do not forward the GitHub Authorization header to the signed log host.
		response, err = c.httpClient.Do(redirectRequest)
		if err != nil {
			return "", err
		}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("GitHub Actions logs returned HTTP %d", response.StatusCode)
	}
	archive, err := io.ReadAll(io.LimitReader(response.Body, maxPlatformResponse+1))
	if err != nil {
		return "", err
	}
	if len(archive) > maxPlatformResponse {
		return "", errors.New("GitHub Actions log archive exceeded the size limit")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		if len(archive) > maxSourceResponse {
			archive = archive[:maxSourceResponse]
		}
		return string(archive), nil
	}
	var logs strings.Builder
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || logs.Len() >= maxSourceResponse {
			continue
		}
		opened, openErr := file.Open()
		if openErr != nil {
			continue
		}
		remaining := int64(maxSourceResponse - logs.Len())
		content, readErr := io.ReadAll(io.LimitReader(opened, remaining))
		opened.Close()
		if readErr != nil {
			continue
		}
		if logs.Len() > 0 {
			logs.WriteByte('\n')
		}
		logs.WriteString(file.Name)
		logs.WriteByte('\n')
		logs.Write(content)
	}
	return logs.String(), nil
}

type githubActionsRun struct {
	ID         int64     `json:"id"`
	RunNumber  int       `json:"run_number"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	HTMLURL    string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	HeadSHA    string    `json:"head_sha"`
}

func mapGitHubActionsRun(run githubActionsRun) domain.CIBuild {
	status := "QUEUED"
	if run.Status == "in_progress" {
		status = "RUNNING"
	} else if run.Status == "completed" {
		switch strings.ToLower(run.Conclusion) {
		case "success":
			status = "SUCCESS"
		case "cancelled":
			status = "CANCELLED"
		case "timed_out":
			status = "TIMED_OUT"
		default:
			status = "FAILURE"
		}
	}
	result := domain.CIBuild{Provider: "GITHUB_ACTIONS", ID: strconv.FormatInt(run.ID, 10), Reference: run.HeadSHA, Number: run.RunNumber, Status: status, URL: run.HTMLURL}
	if !run.CreatedAt.IsZero() {
		result.StartedAt = &run.CreatedAt
	}
	if run.Status == "completed" && !run.UpdatedAt.IsZero() {
		result.CompletedAt = &run.UpdatedAt
		result.DurationSeconds = int(run.UpdatedAt.Sub(run.CreatedAt).Seconds())
	}
	return result
}

func (c *Client) triggerKubernetesJob(ctx context.Context, integration domain.PlatformIntegration, commit domain.CommitMetadata) (domain.CIBuild, error) {
	templateRequest, err := c.kubernetesRequest(ctx, integration, http.MethodGet, c.kubernetesJobPath(integration, ""), nil)
	if err != nil {
		return domain.CIBuild{}, err
	}
	var template map[string]any
	if err := c.doJSON(templateRequest, &template, maxPlatformResponse); err != nil {
		return domain.CIBuild{}, err
	}
	metadata, _ := template["metadata"].(map[string]any)
	labels := sanitizedJobLabels(metadata["labels"])
	job := integrationString(integration.JobName, integration.JenkinsJobName, "sre-copilot-pipeline")
	if !isDNSLabel(job) {
		return domain.CIBuild{}, errors.New("Kubernetes Job jobName must be a lowercase DNS label")
	}
	suffix := commit.SHA
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	if suffix == "" {
		suffix = "manual"
	}
	suffix = sanitizeNamePart(suffix)
	prefix := job + "-" + strings.ToLower(suffix) + "-"
	if len(prefix) > 58 {
		return domain.CIBuild{}, errors.New("Kubernetes generated Job name would exceed 63 characters")
	}
	spec, ok := template["spec"].(map[string]any)
	if !ok {
		return domain.CIBuild{}, errors.New("Kubernetes Job template did not contain a spec")
	}
	delete(spec, "selector")
	delete(spec, "manualSelector")
	if podTemplate, ok := spec["template"].(map[string]any); ok {
		podMetadata, _ := podTemplate["metadata"].(map[string]any)
		if podMetadata == nil {
			podMetadata = make(map[string]any)
			podTemplate["metadata"] = podMetadata
		}
		podMetadata["labels"] = sanitizedJobLabels(podMetadata["labels"])
	}
	payload := map[string]any{
		"apiVersion": "batch/v1", "kind": "Job",
		"metadata": map[string]any{"generateName": prefix, "labels": labels},
		"spec":     spec,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.CIBuild{}, err
	}
	request, err := c.kubernetesRequest(ctx, integration, http.MethodPost, c.kubernetesJobCollectionPath(), bytes.NewReader(body))
	if err != nil {
		return domain.CIBuild{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	var response struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
		return domain.CIBuild{}, err
	}
	if response.Metadata.Name == "" {
		return domain.CIBuild{}, errors.New("Kubernetes API did not return the created Job name")
	}
	return domain.CIBuild{Provider: "KUBERNETES_JOB", ID: response.Metadata.Name, Reference: c.kubernetesJobCollectionPath() + "/" + url.PathEscape(response.Metadata.Name), Status: "QUEUED"}, nil
}

func (c *Client) pollKubernetesJob(ctx context.Context, integration domain.PlatformIntegration, build domain.CIBuild) (domain.CIBuild, error) {
	if build.ID == "" {
		return domain.CIBuild{}, errors.New("Kubernetes Job build ID is missing")
	}
	request, err := c.kubernetesRequest(ctx, integration, http.MethodGet, c.kubernetesJobCollectionPath()+"/"+url.PathEscape(build.ID), nil)
	if err != nil {
		return domain.CIBuild{}, err
	}
	var response struct {
		Status struct {
			Active         int       `json:"active"`
			Succeeded      int       `json:"succeeded"`
			Failed         int       `json:"failed"`
			StartTime      time.Time `json:"startTime"`
			CompletionTime time.Time `json:"completionTime"`
			Conditions     []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	}
	if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
		return domain.CIBuild{}, err
	}
	build.Provider = "KUBERNETES_JOB"
	build.Status = "QUEUED"
	complete, failed := false, false
	for _, condition := range response.Status.Conditions {
		if !strings.EqualFold(condition.Status, "true") {
			continue
		}
		if strings.EqualFold(condition.Type, "Complete") {
			complete = true
		}
		if strings.EqualFold(condition.Type, "Failed") {
			failed = true
		}
	}
	if failed {
		build.Status = "FAILURE"
	} else if complete {
		build.Status = "SUCCESS"
	} else if response.Status.Active > 0 || response.Status.Failed > 0 {
		build.Status = "RUNNING"
	}
	if !response.Status.StartTime.IsZero() {
		build.StartedAt = &response.Status.StartTime
	}
	if !response.Status.CompletionTime.IsZero() {
		build.CompletedAt = &response.Status.CompletionTime
		build.DurationSeconds = int(response.Status.CompletionTime.Sub(response.Status.StartTime).Seconds())
	}
	if build.Failed() {
		if logs, logErr := c.kubernetesPodLogs(ctx, integration, build.ID); logErr == nil {
			build.Logs = logs
		}
	}
	return build, nil
}

func (c *Client) kubernetesPodLogs(ctx context.Context, integration domain.PlatformIntegration, jobName string) (string, error) {
	query := url.Values{}
	query.Set("labelSelector", "job-name="+jobName)
	podsPath := "/api/v1/namespaces/" + url.PathEscape(c.kubernetesNamespace) + "/pods?" + query.Encode()
	request, err := c.kubernetesRequest(ctx, integration, http.MethodGet, podsPath, nil)
	if err != nil {
		return "", err
	}
	var response struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
		return "", err
	}
	if len(response.Items) == 0 || response.Items[0].Metadata.Name == "" {
		return "", errors.New("no pod was found for failed Kubernetes Job")
	}
	logQuery := url.Values{}
	logQuery.Set("allContainers", "true")
	logQuery.Set("tailLines", "500")
	logQuery.Set("limitBytes", strconv.Itoa(maxSourceResponse))
	logPath := "/api/v1/namespaces/" + url.PathEscape(c.kubernetesNamespace) + "/pods/" + url.PathEscape(response.Items[0].Metadata.Name) + "/log?" + logQuery.Encode()
	logRequest, err := c.kubernetesRequest(ctx, integration, http.MethodGet, logPath, nil)
	if err != nil {
		return "", err
	}
	content, err := c.doBytes(logRequest, maxSourceResponse)
	return string(content), err
}

func (c *Client) jenkinsAutomationRequest(ctx context.Context, integration domain.PlatformIntegration, method, suffix string, body io.Reader) (*http.Request, error) {
	raw := integrationString(integration.CIBaseURL, integration.JenkinsURL, "")
	if raw == "" {
		return nil, errors.New("ciBaseUrl is required for Jenkins")
	}
	base, err := safeIntegrationURL(raw, c.allowPrivate)
	if err != nil {
		return nil, err
	}
	job := integrationString(integration.JobName, integration.JenkinsJobName, "sre-copilot-pipeline")
	if strings.ContainsAny(job, "/\\?#") {
		return nil, errors.New("Jenkins job name contains unsupported path characters")
	}
	query := ""
	if index := strings.IndexByte(suffix, '?'); index >= 0 {
		query = suffix[index+1:]
		suffix = suffix[:index]
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/job/" + url.PathEscape(job) + "/" + strings.TrimLeft(suffix, "/")
	base.RawQuery = query
	return c.authenticatedCIRequest(ctx, integration, method, base.String(), body)
}

func (c *Client) githubActionsRequest(ctx context.Context, integration domain.PlatformIntegration, method, operation string, body io.Reader) (*http.Request, error) {
	details, err := c.repositoryDetails(integration)
	if err != nil {
		return nil, err
	}
	if details.provider != "GITHUB" {
		return nil, errors.New("GitHub Actions requires a GitHub repository")
	}
	if raw := strings.TrimSpace(valueOf(integration.CIBaseURL)); raw != "" {
		parsed, parseErr := safeIntegrationURL(raw, false)
		if parseErr != nil || (parsed.Hostname() != "api.github.com" && parsed.Hostname() != "github.com") {
			return nil, errors.New("GitHub Actions ciBaseUrl must use github.com or api.github.com")
		}
	}
	base := *c.githubBase
	base.Path = strings.TrimRight(base.Path, "/") + "/repos/" + url.PathEscape(details.owner) + "/" + url.PathEscape(details.repo) + "/actions"
	workflow := integrationString(integration.JobName, integration.JenkinsJobName, "")
	if workflow == "" || strings.ContainsAny(workflow, "/\\?#") {
		return nil, errors.New("jobName must identify a GitHub Actions workflow")
	}
	switch {
	case operation == "workflow":
		base.Path += "/workflows/" + url.PathEscape(workflow)
	case operation == "dispatch":
		base.Path += "/workflows/" + url.PathEscape(workflow) + "/dispatches"
	case operation == "runs":
		base.Path += "/workflows/" + url.PathEscape(workflow) + "/runs"
	case strings.HasPrefix(operation, "run/"):
		base.Path += "/runs/" + strings.TrimPrefix(operation, "run/")
	default:
		return nil, errors.New("unsupported GitHub Actions operation")
	}
	request, err := http.NewRequestWithContext(ctx, method, base.String(), body)
	if err != nil {
		return nil, err
	}
	c.repositoryHeaders(request, "GITHUB", integration)
	if token := integrationString(integration.CIToken, integration.RepositoryToken, ""); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	return request, nil
}

func (c *Client) kubernetesRequest(ctx context.Context, integration domain.PlatformIntegration, method, suffix string, body io.Reader) (*http.Request, error) {
	// The service account needs only batch/jobs get+create, core/pods get+list,
	// and core/pods/log get in c.kubernetesNamespace. No update/delete/exec
	// permission is required by this read/clone/poll adapter.
	raw := strings.TrimSpace(valueOf(integration.CIBaseURL))
	if raw == "" {
		return nil, errors.New("ciBaseUrl is required for Kubernetes Job")
	}
	base, err := safeIntegrationURL(raw, c.allowPrivate)
	if err != nil {
		return nil, err
	}
	relative, err := url.Parse(suffix)
	if err != nil {
		return nil, errors.New("Kubernetes API path is invalid")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(relative.Path, "/")
	base.RawQuery = relative.RawQuery
	request, err := c.authenticatedCIRequest(ctx, integration, method, base.String(), body)
	if err != nil {
		return nil, err
	}
	if request.Header.Get("Authorization") == "" && c.kubernetesTokenFile != "" {
		token, tokenErr := readBoundedToken(c.kubernetesTokenFile)
		if tokenErr != nil {
			return nil, tokenErr
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return request, nil
}

func readBoundedToken(fileName string) (string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read Kubernetes service account token: %w", err)
	}
	defer file.Close()
	const maxTokenBytes = 64 << 10
	content, err := io.ReadAll(io.LimitReader(file, maxTokenBytes+1))
	if err != nil {
		return "", fmt.Errorf("read Kubernetes service account token: %w", err)
	}
	if len(content) > maxTokenBytes {
		return "", errors.New("Kubernetes service account token exceeds 64 KiB")
	}
	return strings.TrimSpace(string(content)), nil
}

func (c *Client) authenticatedCIRequest(ctx context.Context, integration domain.PlatformIntegration, method, requestURL string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, err
	}
	username := integrationString(integration.CIUsername, integration.JenkinsUsername, "")
	token := integrationString(integration.CIToken, integration.JenkinsAPIToken, "")
	if username != "" && token != "" {
		request.SetBasicAuth(username, token)
	} else if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request, nil
}

func (c *Client) validatedCIReference(integration domain.PlatformIntegration, raw string) (*url.URL, error) {
	reference, err := safeIntegrationURL(raw, c.allowPrivate)
	if err != nil {
		return nil, err
	}
	base, err := safeIntegrationURL(integrationString(integration.CIBaseURL, integration.JenkinsURL, ""), c.allowPrivate)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(reference.Scheme, base.Scheme) || !strings.EqualFold(reference.Host, base.Host) {
		return nil, errors.New("CI status URL changed origin")
	}
	return reference, nil
}

func (c *Client) kubernetesJobCollectionPath() string {
	return "/apis/batch/v1/namespaces/" + url.PathEscape(c.kubernetesNamespace) + "/jobs"
}

func (c *Client) kubernetesJobPath(integration domain.PlatformIntegration, suffix string) string {
	job := integrationString(integration.JobName, integration.JenkinsJobName, "sre-copilot-pipeline")
	return c.kubernetesJobCollectionPath() + "/" + url.PathEscape(job) + suffix
}

func safeNamespace(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "sre-copilot"
	}
	for _, character := range trimmed {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return "sre-copilot"
		}
	}
	return trimmed
}

func sanitizedJobLabels(raw any) map[string]any {
	input, _ := raw.(map[string]any)
	result := make(map[string]any, len(input))
	for key, value := range input {
		switch key {
		case "controller-uid", "batch.kubernetes.io/controller-uid", "job-name", "batch.kubernetes.io/job-name":
			continue
		default:
			result[key] = value
		}
	}
	return result
}

func isDNSLabel(value string) bool {
	if len(value) == 0 || len(value) > 49 || !((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= '0' && value[0] <= '9')) {
		return false
	}
	for index, character := range value {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-'
		if !valid || (character == '-' && (index == 0 || index == len(value)-1)) {
			return false
		}
	}
	return true
}

func sanitizeNamePart(value string) string {
	var result strings.Builder
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			result.WriteRune(character)
		}
		if result.Len() == 8 {
			break
		}
	}
	if result.Len() == 0 {
		return "manual"
	}
	return result.String()
}
