package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/models"
	"github.com/anandhuremanan/cicd-engine/internal/service"
)

// Server provides the HTTP REST API and SSE stream server for the CI/CD Web Dashboard.
type Server struct {
	pipelineSvc service.PipelineService
	mux         *http.ServeMux
	distDir     string
}

// NewServer constructs an API server instance with route bindings.
func NewServer(svc service.PipelineService, distDir string) *Server {
	if distDir == "" {
		distDir = filepath.Join(".", "frontend", "dist")
	}

	s := &Server{
		pipelineSvc: svc,
		mux:         http.NewServeMux(),
		distDir:     distDir,
	}

	s.routes()
	return s
}

// Handler returns the HTTP handler wrapped with CORS headers for development flexibility.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/builds", s.handleBuilds)
	s.mux.HandleFunc("/api/builds/", s.handleBuildSubpath)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/doctor", s.handleDoctor)

	// SPA Static File Fallback
	s.mux.HandleFunc("/", s.handleStaticSPA)
}

func (s *Server) handleBuilds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		history, err := s.pipelineSvc.History()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get history: %v", err), http.StatusInternalServerError)
			return
		}
		if history == nil {
			history = []models.Job{}
		}
		writeJSON(w, http.StatusOK, history)

	case http.MethodPost:
		var req cli.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
			return
		}

		if req.PipelineContent != "" {
			tmpFile, err := os.CreateTemp("", "cicd-pipeline-*.yaml")
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to write pipeline temp file: %v", err), http.StatusInternalServerError)
				return
			}
			if _, err := tmpFile.WriteString(req.PipelineContent); err != nil {
				tmpFile.Close()
				http.Error(w, fmt.Sprintf("failed to write pipeline content: %v", err), http.StatusInternalServerError)
				return
			}
			tmpFile.Close()
			req.PipelineFile = tmpFile.Name()
		}

		if req.RepositoryURL == "" && req.PipelineFile == "" {
			http.Error(w, "repository_url, pipeline_file, or pipeline_content must be specified", http.StatusBadRequest)
			return
		}

		jobID, err := s.pipelineSvc.RunAsync(r.Context(), req)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to trigger build: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"job_id":  jobID,
			"status":  "RUNNING",
			"message": "Build job started successfully",
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleBuildSubpath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/builds/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "job ID required", http.StatusBadRequest)
		return
	}

	jobID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		s.handleGetJob(w, r, jobID)
	case "logs":
		s.handleGetJobLogs(w, r, jobID)
	case "stream":
		s.handleStreamJobLogs(w, r, jobID)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request, jobID string) {
	job, err := s.pipelineSvc.Status(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleGetJobLogs(w http.ResponseWriter, r *http.Request, jobID string) {
	logEntries, err := s.pipelineSvc.Logs(jobID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read logs: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, logEntries)
}

func (s *Server) handleStreamJobLogs(w http.ResponseWriter, r *http.Request, jobID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 1. Send past log entries first
	pastLogs, err := s.pipelineSvc.Logs(jobID)
	if err == nil {
		for _, entry := range pastLogs {
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
		}
		flusher.Flush()
	}

	// 2. Subscribe to live stream
	ch, unsubscribe := s.pipelineSvc.Subscribe(jobID)
	defer unsubscribe()

	ctx := r.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
			flusher.Flush()

		case <-ticker.C:
			// Send heartbeat & current job status
			job, err := s.pipelineSvc.Status(jobID)
			if err == nil {
				jobData, _ := json.Marshal(job)
				fmt.Fprintf(w, "event: status\ndata: %s\n\n", jobData)
				flusher.Flush()
			}
		}
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	running, err := s.pipelineSvc.RunningJobs()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get active status: %v", err), http.StatusInternalServerError)
		return
	}
	if running == nil {
		running = []models.Job{}
	}
	writeJSON(w, http.StatusOK, running)
}

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	diag, err := s.pipelineSvc.Doctor(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("doctor diagnostic failed: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, diag)
}

func (s *Server) handleStaticSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(s.distDir, filepath.Clean(r.URL.Path))
	info, err := os.Stat(path)

	// If requested static file exists and is not a directory, serve it directly
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}

	// Fallback to index.html for SPA client-side routing
	indexPath := filepath.Join(s.distDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	// Fallback response if dist/ contains no build yet
	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, `<!text/html><html><head><title>CI/CD Dashboard</title></head><body style="font-family:sans-serif;padding:2rem;"><h2>CI/CD Engine Server Active</h2><p>Frontend assets building. API endpoints are live at <code>/api/*</code>.</p></body></html>`)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ListenAndServe launches the HTTP server on specified address.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}
	return srv.ListenAndServe()
}

// ListenAndServeContext launches the HTTP server with context cancellation.
func (s *Server) ListenAndServeContext(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
