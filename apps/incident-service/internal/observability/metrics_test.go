package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMetricsExposeAlertContractAndNormalizedRoute(t *testing.T) {
	poolConfig, err := pgxpool.ParseConfig("postgres://user:password@127.0.0.1:1/database?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	metrics := New("incident-service", pool)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	requestResponse := httptest.NewRecorder()
	metrics.Middleware(mux).ServeHTTP(requestResponse, httptest.NewRequest(http.MethodGet, "/items/42", nil))

	scrape := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/actuator/prometheus", nil))
	body := scrape.Body.String()
	for _, expected := range []string{
		`http_server_requests_total{application="incident-service",method="GET",route="GET /items/{id}",status="204"} 1`,
		`db_pool_waiting_requests{application="incident-service"} 0`,
		`db_pool_empty_acquire_total{application="incident-service"} 0`,
		`db_pool_max_connections{application="incident-service"}`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics scrape does not contain %q:\n%s", expected, body)
		}
	}
}
