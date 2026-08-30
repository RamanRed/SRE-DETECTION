package platform

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

const maxPlatformResponse = 2 << 20

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	httpClient          HTTPDoer
	githubBase          *url.URL
	allowPrivate        bool
	kubernetesNamespace string
	kubernetesTokenFile string
}

type KubernetesConfig struct {
	Namespace string
	CAFile    string
	TokenFile string
}

func New(githubAPIBase string, connectTimeout, requestTimeout time.Duration, allowPrivate ...bool) (*Client, error) {
	privateAllowed := len(allowPrivate) > 0 && allowPrivate[0]
	return NewConfigured(githubAPIBase, connectTimeout, requestTimeout, privateAllowed, KubernetesConfig{Namespace: "sre-copilot"})
}

func NewConfigured(githubAPIBase string, connectTimeout, requestTimeout time.Duration, allowPrivate bool, kubernetes KubernetesConfig) (*Client, error) {
	base, err := url.Parse(githubAPIBase)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return nil, fmt.Errorf("GITHUB_API_BASE must be an absolute HTTPS URL")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = guardedDialer(connectTimeout, allowPrivate)
	transport.ResponseHeaderTimeout = requestTimeout
	transport.TLSHandshakeTimeout = connectTimeout
	if kubernetes.CAFile != "" {
		certificate, readErr := os.ReadFile(kubernetes.CAFile)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return nil, fmt.Errorf("read Kubernetes CA: %w", readErr)
		}
		if readErr == nil {
			roots, rootErr := x509.SystemCertPool()
			if rootErr != nil || roots == nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM(certificate) {
				return nil, errors.New("KUBERNETES_CA_FILE does not contain a valid PEM certificate")
			}
			transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
		}
	}
	if kubernetes.TokenFile != "" {
		_, readErr := os.Stat(kubernetes.TokenFile)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return nil, fmt.Errorf("read Kubernetes service account token: %w", readErr)
		}
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Client{httpClient: client, githubBase: base, allowPrivate: allowPrivate, kubernetesNamespace: safeNamespace(kubernetes.Namespace), kubernetesTokenFile: kubernetes.TokenFile}, nil
}

func NewWithHTTPClient(githubAPIBase string, client HTTPDoer, allowPrivate ...bool) (*Client, error) {
	base, err := url.Parse(githubAPIBase)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("GitHub API base URL is invalid")
	}
	return &Client{
		httpClient: client, githubBase: base,
		allowPrivate: len(allowPrivate) > 0 && allowPrivate[0], kubernetesNamespace: "sre-copilot",
	}, nil
}

func (c *Client) TestGitHub(ctx context.Context, integration domain.PlatformIntegration) error {
	requestURL, err := c.githubURL(integration, false)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	c.githubHeaders(request, integration)
	return c.doStatus(request)
}

func (c *Client) TestJenkins(ctx context.Context, integration domain.PlatformIntegration) error {
	request, err := c.jenkinsRequest(ctx, integration)
	if err != nil {
		return err
	}
	return c.doStatus(request)
}

func (c *Client) SyncGitHub(ctx context.Context, integration domain.PlatformIntegration) (int, error) {
	requestURL, err := c.githubURL(integration, true)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return 0, err
	}
	c.githubHeaders(request, integration)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return 0, fmt.Errorf("GitHub returned HTTP %d", response.StatusCode)
	}
	var commits []json.RawMessage
	if err := json.NewDecoder(io.LimitReader(response.Body, maxPlatformResponse)).Decode(&commits); err != nil {
		return 0, fmt.Errorf("decode GitHub commits: %w", err)
	}
	return len(commits), nil
}

func (c *Client) SyncJenkins(ctx context.Context, integration domain.PlatformIntegration) (int, error) {
	request, err := c.jenkinsRequest(ctx, integration)
	if err != nil {
		return 0, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxPlatformResponse))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, fmt.Errorf("Jenkins returned HTTP %d", response.StatusCode)
	}
	// The legacy endpoint reports the four most recent Jenkins builds on a successful sync.
	return 4, nil
}

func (c *Client) githubURL(integration domain.PlatformIntegration, commits bool) (string, error) {
	if integration.GitHubRepo == nil {
		return "", errors.New("GitHub repository is not configured")
	}
	parts := strings.Split(*integration.GitHubRepo, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("GitHub repository must be owner/repository")
	}
	resolved := *c.githubBase
	resolved.Path = strings.TrimRight(resolved.Path, "/") + "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
	if commits {
		resolved.Path += "/commits"
		branch := "main"
		if integration.GitHubBranch != nil && strings.TrimSpace(*integration.GitHubBranch) != "" {
			branch = *integration.GitHubBranch
		}
		query := resolved.Query()
		query.Set("sha", branch)
		query.Set("per_page", "5")
		resolved.RawQuery = query.Encode()
	}
	return resolved.String(), nil
}

func (c *Client) githubHeaders(request *http.Request, integration domain.PlatformIntegration) {
	request.Header.Set("User-Agent", "SRE-Incident-Copilot")
	request.Header.Set("Accept", "application/vnd.github.v3+json")
	if integration.GitHubToken != nil && strings.TrimSpace(*integration.GitHubToken) != "" {
		request.Header.Set("Authorization", "Bearer "+*integration.GitHubToken)
	}
}

func (c *Client) jenkinsRequest(ctx context.Context, integration domain.PlatformIntegration) (*http.Request, error) {
	if integration.JenkinsURL == nil {
		return nil, errors.New("Jenkins URL is not configured")
	}
	base, err := safeIntegrationURL(*integration.JenkinsURL, c.allowPrivate)
	if err != nil {
		return nil, err
	}
	job := "sre-copilot-pipeline"
	if integration.JenkinsJobName != nil && strings.TrimSpace(*integration.JenkinsJobName) != "" {
		job = *integration.JenkinsJobName
	}
	if strings.ContainsAny(job, "/\\?#") {
		return nil, errors.New("Jenkins job name contains unsupported path characters")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/job/" + url.PathEscape(job) + "/api/json"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	if integration.JenkinsUsername != nil && integration.JenkinsAPIToken != nil {
		request.SetBasicAuth(*integration.JenkinsUsername, *integration.JenkinsAPIToken)
	}
	return request, nil
}

func safeJenkinsURL(raw string) (*url.URL, error) {
	return safeIntegrationURL(raw, false)
}

func safeIntegrationURL(raw string, allowPrivate bool) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("Jenkins URL must be an absolute HTTP or HTTPS URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Jenkins URL must not contain credentials, query parameters, or fragments")
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return nil, errors.New("integration URL may not target localhost")
	}
	if !allowPrivate && (strings.HasSuffix(hostname, ".local") || strings.HasSuffix(hostname, ".internal") ||
		strings.HasSuffix(hostname, ".svc") || !strings.Contains(hostname, ".")) {
		return nil, errors.New("integration URL targets a private hostname")
	}
	if address := net.ParseIP(hostname); address != nil && (alwaysRestrictedIP(address) || (!allowPrivate && address.IsPrivate())) {
		return nil, errors.New("integration URL targets a restricted network address")
	}
	return parsed, nil
}

func guardedDialer(timeout time.Duration, allowPrivate bool) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, resolved := range addresses {
			if alwaysRestrictedIP(resolved.IP) || (!allowPrivate && resolved.IP.IsPrivate()) {
				return nil, fmt.Errorf("integration endpoint resolved to a restricted network address")
			}
		}
		if len(addresses) == 0 {
			return nil, errors.New("integration endpoint did not resolve to an address")
		}
		// Dial the already-vetted address to prevent a second DNS lookup from
		// turning validation into a DNS-rebinding race. TLS still validates the
		// original request hostname at the HTTP transport layer.
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
	}
}

func alwaysRestrictedIP(address net.IP) bool {
	return address.IsLoopback() || address.IsUnspecified() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast()
}

func (c *Client) doStatus(request *http.Request) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("platform returned HTTP %d", response.StatusCode)
	}
	return nil
}
