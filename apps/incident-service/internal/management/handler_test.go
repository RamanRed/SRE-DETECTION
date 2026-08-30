package management

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadinessReflectsDatabaseAndLivenessDoesNot(t *testing.T) {
	health := &fakeHealth{err: errors.New("database offline")}
	handler := New(health, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("metric 1\n"))
	}), "incident-service", "test")

	readiness := httptest.NewRecorder()
	handler.ServeHTTP(readiness, httptest.NewRequest(http.MethodGet, "/actuator/health/readiness", nil))
	if readiness.Code != http.StatusServiceUnavailable || !strings.Contains(readiness.Body.String(), `"status":"DOWN"`) {
		t.Fatalf("readiness = %d %s", readiness.Code, readiness.Body.String())
	}
	liveness := httptest.NewRecorder()
	handler.ServeHTTP(liveness, httptest.NewRequest(http.MethodGet, "/actuator/health/liveness", nil))
	if liveness.Code != http.StatusOK || !strings.Contains(liveness.Body.String(), `"status":"UP"`) {
		t.Fatalf("liveness = %d %s", liveness.Code, liveness.Body.String())
	}
}

type fakeHealth struct{ err error }

func (f *fakeHealth) Ping(context.Context) error { return f.err }
