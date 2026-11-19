package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var embeddedDist embed.FS

func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(embeddedDist, "dist")
	if err == nil {
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(embeddedReviewStudioHTML))
}

const embeddedReviewStudioHTML = `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
  <meta charset="UTF-8">
  <title>videoblur // Autonomous Legal Redaction Review Studio</title>
</head>
<body style="background:#020617;color:#f8fafc;font-family:sans-serif;padding:2rem;">
  <h2>videoblur Review Studio</h2>
  <p>Live Review Studio is ready.</p>
</body>
</html>`
