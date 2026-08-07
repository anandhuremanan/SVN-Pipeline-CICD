package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anandhuremanan/cicd-engine/internal/api"
	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/service"
)

func TestAPIServer_Endpoints(t *testing.T) {
	svc := service.NewDefaultPipelineService(nil, nil, nil, nil, nil)
	srv := api.NewServer(svc, "")

	req := httptest.NewRequest(http.MethodGet, "/api/builds", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected GET /api/builds status 200, got %d", w.Code)
	}

	reqDoctor := httptest.NewRequest(http.MethodGet, "/api/doctor", nil)
	wDoctor := httptest.NewRecorder()
	srv.Handler().ServeHTTP(wDoctor, reqDoctor)

	if wDoctor.Code != http.StatusOK {
		t.Errorf("expected GET /api/doctor status 200, got %d", wDoctor.Code)
	}
}

func TestAPIServer_PostBuild(t *testing.T) {
	svc := service.NewDefaultPipelineService(nil, nil, nil, nil, nil)
	srv := api.NewServer(svc, "")

	payload := cli.RunRequest{
		RepositoryURL: "https://svn.example.com/repos/test",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/builds", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202 Accepted, got %d", w.Code)
	}
}
