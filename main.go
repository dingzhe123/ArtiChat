package main

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"ai-article-site/config"
	"ai-article-site/handlers"
	"ai-article-site/middleware"
)

func main() {
	cfg := config.Load()

	// Parse templates — base.html is the layout, others define the "content" block.
	tmplPath := filepath.Join("templates", "*.html")
	tmpl, err := template.ParseGlob(tmplPath)
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	// Handlers
	homeH := &handlers.HomeHandler{Tmpl: tmpl}
	articleH := &handlers.ArticleHandler{Tmpl: tmpl}
	adminH := &handlers.AdminHandler{Tmpl: tmpl}

	// Router — using Go 1.22+ enhanced ServeMux with method-based routing.
	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("GET /", homeH.ServeHTTP)
	mux.HandleFunc("GET /articles", articleH.ServeHTTP)
	mux.HandleFunc("GET /admin", adminH.ServeHTTP)

	// Static files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Apply middleware: CORS → Logger → Router
	handler := middleware.Logger(middleware.CORS(mux))

	log.Printf("🚀 Server starting on http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
