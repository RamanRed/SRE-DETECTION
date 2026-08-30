package platform

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

func TestGitHubSyncEncodesInputsAndAddsBearerToken(t *testing.T) {
	var captured *http.Request
	client, err := NewWithHTTPClient("https://api.github.test", roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		captured = request
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{},{}]`)), Header: make(http.Header)}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	repo, branch, token := "owner/repo", "feature/a b", "secret"
	count, err := client.SyncGitHub(t.Context(), domain.PlatformIntegration{GitHubRepo: &repo, GitHubBranch: &branch, GitHubToken: &token})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || captured.URL.Query().Get("sha") != branch || captured.URL.Query().Get("per_page") != "5" {
		t.Fatalf("request URL=%s count=%d", captured.URL, count)
	}
	if captured.Header.Get("Authorization") != "Bearer secret" || captured.Header.Get("User-Agent") != "SRE-Incident-Copilot" {
		t.Fatalf("headers = %#v", captured.Header)
	}
}

func TestJenkinsRejectsLoopbackAndBuildsSafeDefaultURL(t *testing.T) {
	loopback := "http://127.0.0.1:8080"
	client, err := NewWithHTTPClient("https://api.github.test", roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.TestJenkins(t.Context(), domain.PlatformIntegration{JenkinsURL: &loopback}); err == nil {
		t.Fatal("loopback Jenkins URL was accepted")
	}
	publicURL, job, username, token := "https://jenkins.example.com/", "sre-copilot-pipeline", "ci", "token"
	var captured *http.Request
	client.httpClient = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		captured = request
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	if err := client.TestJenkins(t.Context(), domain.PlatformIntegration{
		JenkinsURL: &publicURL, JenkinsJobName: &job, JenkinsUsername: &username, JenkinsAPIToken: &token,
	}); err != nil {
		t.Fatal(err)
	}
	if captured.URL.String() != "https://jenkins.example.com/job/sre-copilot-pipeline/api/json" {
		t.Fatalf("Jenkins URL = %s", captured.URL)
	}
	if value := captured.Header.Get("Authorization"); !strings.HasPrefix(value, "Basic ") {
		t.Fatalf("basic auth header = %q", value)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }
