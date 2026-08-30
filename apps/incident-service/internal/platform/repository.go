package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

const maxSourceResponse = 32 << 10

type repositoryDetails struct {
	provider string
	owner    string
	repo     string
	fullPath string
}

func (c *Client) TestRepository(ctx context.Context, integration domain.PlatformIntegration) error {
	details, err := c.repositoryDetails(integration)
	if err != nil {
		return err
	}
	requestURL, err := c.repositoryAPIURL(details, "repository", "", "")
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	c.repositoryHeaders(request, details.provider, integration)
	return c.doStatus(request)
}

func (c *Client) LatestCommit(ctx context.Context, integration domain.PlatformIntegration) (domain.CommitMetadata, error) {
	details, err := c.repositoryDetails(integration)
	if err != nil {
		return domain.CommitMetadata{}, err
	}
	branch := integrationString(integration.TargetBranch, integration.GitHubBranch, "main")
	requestURL, err := c.repositoryAPIURL(details, "commit", branch, "")
	if err != nil {
		return domain.CommitMetadata{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return domain.CommitMetadata{}, err
	}
	c.repositoryHeaders(request, details.provider, integration)
	var commit domain.CommitMetadata
	commit.Provider = details.provider
	commit.Repository = strings.TrimSpace(valueOf(integration.RepositoryURL))
	commit.Branch = branch
	switch details.provider {
	case "GITHUB":
		var response struct {
			SHA     string `json:"sha"`
			HTMLURL string `json:"html_url"`
			Commit  struct {
				Message string `json:"message"`
				Author  struct {
					Name string    `json:"name"`
					Date time.Time `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		}
		if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
			return domain.CommitMetadata{}, err
		}
		commit.SHA, commit.Message, commit.Author, commit.URL = response.SHA, response.Commit.Message, response.Commit.Author.Name, response.HTMLURL
		if !response.Commit.Author.Date.IsZero() {
			commit.CommittedAt = &response.Commit.Author.Date
		}
	case "GITLAB":
		var response struct {
			ID            string    `json:"id"`
			Message       string    `json:"message"`
			AuthorName    string    `json:"author_name"`
			WebURL        string    `json:"web_url"`
			CommittedDate time.Time `json:"committed_date"`
		}
		if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
			return domain.CommitMetadata{}, err
		}
		commit.SHA, commit.Message, commit.Author, commit.URL = response.ID, response.Message, response.AuthorName, response.WebURL
		if !response.CommittedDate.IsZero() {
			commit.CommittedAt = &response.CommittedDate
		}
	case "BITBUCKET":
		var response struct {
			Values []struct {
				Hash    string    `json:"hash"`
				Message string    `json:"message"`
				Date    time.Time `json:"date"`
				Author  struct {
					Raw string `json:"raw"`
				} `json:"author"`
				Links struct {
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				} `json:"links"`
			} `json:"values"`
		}
		if err := c.doJSON(request, &response, maxPlatformResponse); err != nil {
			return domain.CommitMetadata{}, err
		}
		if len(response.Values) == 0 {
			return domain.CommitMetadata{}, errors.New("Bitbucket returned no commits")
		}
		item := response.Values[0]
		commit.SHA, commit.Message, commit.Author, commit.URL = item.Hash, item.Message, item.Author.Raw, item.Links.HTML.Href
		if !item.Date.IsZero() {
			commit.CommittedAt = &item.Date
		}
	}
	if strings.TrimSpace(commit.SHA) == "" {
		return domain.CommitMetadata{}, errors.New("repository response did not include a commit SHA")
	}
	return commit, nil
}

func (c *Client) FetchSource(ctx context.Context, integration domain.PlatformIntegration, commitSHA, sourcePath string, maxBytes int) (domain.SourceSnippet, error) {
	details, err := c.repositoryDetails(integration)
	if err != nil {
		return domain.SourceSnippet{}, err
	}
	cleaned, err := cleanSourcePath(sourcePath)
	if err != nil {
		return domain.SourceSnippet{}, err
	}
	if maxBytes <= 0 || maxBytes > maxSourceResponse {
		maxBytes = maxSourceResponse
	}
	requestURL, err := c.repositoryAPIURL(details, "source", commitSHA, cleaned)
	if err != nil {
		return domain.SourceSnippet{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return domain.SourceSnippet{}, err
	}
	c.repositoryHeaders(request, details.provider, integration)
	if details.provider == "GITHUB" {
		request.Header.Set("Accept", "application/vnd.github.raw+json")
	}
	content, err := c.doBytes(request, maxBytes)
	if err != nil {
		return domain.SourceSnippet{}, err
	}
	endLine := int32(1 + strings.Count(string(content), "\n"))
	return domain.SourceSnippet{Path: cleaned, Content: string(content), StartLine: 1, EndLine: endLine}, nil
}

func (c *Client) repositoryDetails(integration domain.PlatformIntegration) (repositoryDetails, error) {
	provider := strings.ToUpper(integrationString(integration.RepositoryProvider, nil, "GITHUB"))
	raw := strings.TrimSpace(valueOf(integration.RepositoryURL))
	if raw == "" && integration.GitHubRepo != nil {
		raw = "https://github.com/" + strings.TrimSpace(*integration.GitHubRepo)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return repositoryDetails{}, errors.New("repository URL must be an absolute HTTPS URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return repositoryDetails{}, errors.New("repository URL must not include credentials, query parameters, or fragments")
	}
	host := strings.ToLower(parsed.Hostname())
	parts := strings.Split(strings.TrimSuffix(strings.Trim(parsed.EscapedPath(), "/"), ".git"), "/")
	decoded := make([]string, 0, len(parts))
	for _, part := range parts {
		value, decodeErr := url.PathUnescape(part)
		if decodeErr != nil || value == "" || value == "." || value == ".." {
			return repositoryDetails{}, errors.New("repository URL path is invalid")
		}
		decoded = append(decoded, value)
	}
	if len(decoded) < 2 {
		return repositoryDetails{}, errors.New("repository URL must identify an owner and repository")
	}
	details := repositoryDetails{provider: provider, owner: decoded[len(decoded)-2], repo: decoded[len(decoded)-1], fullPath: strings.Join(decoded, "/")}
	switch provider {
	case "GITHUB":
		if host != "github.com" {
			return repositoryDetails{}, errors.New("GitHub repository URL must use github.com")
		}
		if len(decoded) != 2 {
			return repositoryDetails{}, errors.New("GitHub repository URL must be https://github.com/owner/repository")
		}
	case "GITLAB":
		if host != "gitlab.com" {
			return repositoryDetails{}, errors.New("GitLab repository URL must use gitlab.com")
		}
	case "BITBUCKET":
		if host != "bitbucket.org" || len(decoded) != 2 {
			return repositoryDetails{}, errors.New("Bitbucket repository URL must be https://bitbucket.org/workspace/repository")
		}
	default:
		return repositoryDetails{}, errors.New("unsupported repository provider")
	}
	return details, nil
}

func (c *Client) repositoryAPIURL(details repositoryDetails, operation, reference, sourcePath string) (string, error) {
	var result *url.URL
	var err error
	switch details.provider {
	case "GITHUB":
		base := *c.githubBase
		appendURLPath(&base, "repos", details.owner, details.repo)
		switch operation {
		case "commit":
			appendURLPath(&base, "commits", reference)
		case "source":
			appendURLPath(&base, append([]string{"contents"}, strings.Split(sourcePath, "/")...)...)
			query := base.Query()
			query.Set("ref", reference)
			base.RawQuery = query.Encode()
		}
		result = &base
	case "GITLAB":
		result = &url.URL{
			Scheme: "https", Host: "gitlab.com",
			Path: "/api/v4/projects/" + details.fullPath, RawPath: "/api/v4/projects/" + url.PathEscape(details.fullPath),
		}
		switch operation {
		case "commit":
			result.Path += "/repository/commits/" + reference
			result.RawPath += "/repository/commits/" + url.PathEscape(reference)
		case "source":
			result.Path += "/repository/files/" + sourcePath + "/raw"
			result.RawPath += "/repository/files/" + url.PathEscape(sourcePath) + "/raw"
			query := result.Query()
			query.Set("ref", reference)
			result.RawQuery = query.Encode()
		}
	case "BITBUCKET":
		result, err = url.Parse("https://api.bitbucket.org/2.0")
		if err == nil {
			appendURLPath(result, "repositories", details.owner, details.repo)
			switch operation {
			case "commit":
				appendURLPath(result, "commits", reference)
				query := result.Query()
				query.Set("pagelen", "1")
				result.RawQuery = query.Encode()
			case "source":
				appendURLPath(result, append([]string{"src", reference}, strings.Split(sourcePath, "/")...)...)
			}
		}
	}
	if err != nil || result == nil {
		return "", errors.New("could not construct repository API URL")
	}
	return result.String(), nil
}

func (c *Client) repositoryHeaders(request *http.Request, provider string, integration domain.PlatformIntegration) {
	request.Header.Set("User-Agent", "SRE-Incident-Copilot")
	token := integrationString(integration.RepositoryToken, integration.GitHubToken, "")
	if token == "" {
		return
	}
	switch provider {
	case "GITLAB":
		request.Header.Set("PRIVATE-TOKEN", token)
	default:
		request.Header.Set("Authorization", "Bearer "+token)
	}
}

func (c *Client) doJSON(request *http.Request, target any, maxBytes int64) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("platform returned HTTP %d", response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(content)) > maxBytes {
		return fmt.Errorf("platform response exceeds %d bytes", maxBytes)
	}
	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("decode platform response: %w", err)
	}
	return nil
}

func (c *Client) doBytes(request *http.Request, maxBytes int) ([]byte, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("platform returned HTTP %d", response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, int64(maxBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxBytes {
		return nil, fmt.Errorf("platform response exceeds %s bytes", strconv.Itoa(maxBytes))
	}
	return content, nil
}

func cleanSourcePath(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	cleaned := path.Clean(strings.TrimPrefix(trimmed, "./"))
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") || strings.ContainsRune(cleaned, '\x00') {
		return "", errors.New("source path must be repository-relative")
	}
	return cleaned, nil
}

func appendURLPath(target *url.URL, segments ...string) {
	decoded := strings.TrimRight(target.Path, "/")
	escaped := strings.TrimRight(target.EscapedPath(), "/")
	for _, segment := range segments {
		decoded += "/" + segment
		escaped += "/" + url.PathEscape(segment)
	}
	target.Path = decoded
	target.RawPath = escaped
}

func integrationString(primary, legacy *string, fallback string) string {
	if primary != nil && strings.TrimSpace(*primary) != "" {
		return strings.TrimSpace(*primary)
	}
	if legacy != nil && strings.TrimSpace(*legacy) != "" {
		return strings.TrimSpace(*legacy)
	}
	return fallback
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
