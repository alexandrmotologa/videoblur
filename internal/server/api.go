package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/alexandrmotologa/videoblur/internal/audit"
	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/media"
)

// Server coordinates HTTP API endpoints and review studio assets.
type Server struct {
	port       int
	videoPath  string
	auditPath  string
	cfg        *config.Config
	clients    map[chan string]bool
	clientsMu  sync.Mutex
	progressMu sync.RWMutex
	lastProgress string
}

// NewServer initializes the review studio server.
func NewServer(port int, videoPath, auditPath string, cfg *config.Config) *Server {
	if port <= 0 {
		port = 8181
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Server{
		port:      port,
		videoPath: videoPath,
		auditPath: auditPath,
		cfg:       cfg,
		clients:   make(map[chan string]bool),
	}
}

// Start launches the local HTTP review studio.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/audit", s.handleAudit)
	mux.HandleFunc("/api/video", s.handleVideoStream)
	mux.HandleFunc("/api/certify", s.handleCertify)
	mux.HandleFunc("/api/events", s.handleSSE)
	mux.HandleFunc("/api/scan", s.handleScan)

	// Web UI handler (serves embedded UI or built-in single-file studio)
	mux.HandleFunc("/", s.handleWebUI)

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	fmt.Printf("\n>>> videoblur Review Studio running at http://%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service":   "videoblur",
		"version":   "1.0.0",
		"status":    "healthy",
		"video":     s.videoPath,
		"audit":     s.auditPath,
		"standards": []string{"CJIS", "HIPAA", "FOIA", "GDPR"},
	})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.auditPath == "" {
		http.Error(w, `{"error":"no audit manifest specified"}`, http.StatusNotFound)
		return
	}

	data, err := os.ReadFile(s.auditPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"manifest not found: %s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleVideoStream(w http.ResponseWriter, r *http.Request) {
	if s.videoPath == "" {
		http.Error(w, "no video loaded", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, s.videoPath)
}

func (s *Server) handleCertify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.auditPath == "" {
		http.Error(w, `{"error":"audit path not configured"}`, http.StatusBadRequest)
		return
	}

	res, err := audit.VerifyManifestFile(s.auditPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	messageChan := make(chan string, 16)
	s.clientsMu.Lock()
	s.clients[messageChan] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, messageChan)
		close(messageChan)
		s.clientsMu.Unlock()
	}()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// BroadcastSSE sends a progress message to all connected SSE clients.
func (s *Server) BroadcastSSE(msg string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.videoPath == "" {
		http.Error(w, `{"error":"no video path specified"}`, http.StatusBadRequest)
		return
	}

	info, err := media.ProbeMedia(s.videoPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"media_info": info,
		"scan_status": "ready",
	})
}
