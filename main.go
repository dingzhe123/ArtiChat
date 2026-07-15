package main

import (
	"html/template"
	"log"
	"net/http"

	"ai-article-site/config"
	"ai-article-site/handlers"
	"ai-article-site/middleware"
	"ai-article-site/services"
)

// parseSet parses base.html plus the given content template files into a
// single template set.  Each set gets its own "content" definition, avoiding
// the name collision you get with ParseGlob.
func parseSet(contentFiles ...string) *template.Template {
	paths := append([]string{"templates/base.html"}, contentFiles...)
	return template.Must(template.ParseFiles(paths...))
}

func main() {
	cfg := config.Load()

	// Initialize services
	articleSvc, err := services.NewArticleService(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to init article service: %v", err)
	}
	defer articleSvc.Close()

	// Per-handler template sets — each set includes base.html plus exactly
	// one content template, so {{block "content"}} resolves to the right page.
	homeTmpl := parseSet("templates/home.html")
	articlesListTmpl := parseSet("templates/article_list.html")
	articlesDetailTmpl := parseSet("templates/article_detail.html")
	adminTmpl := parseSet("templates/admin.html")

	// Handlers
	homeH := &handlers.HomeHandler{Tmpl: homeTmpl}
	articleH := &handlers.ArticleHandler{
		ListTmpl:   articlesListTmpl,
		DetailTmpl: articlesDetailTmpl,
		Service:    articleSvc,
	}
	adminH := &handlers.AdminHandler{Tmpl: adminTmpl, Service: articleSvc}

	// Basic Auth wrapper for admin routes
	auth := handlers.BasicAuth(cfg.AdminUser, cfg.AdminPass)

	// Router — using Go 1.22+ enhanced ServeMux with method-based routing.
	mux := http.NewServeMux()

	// Public pages
	mux.HandleFunc("GET /", homeH.ServeHTTP)
	mux.HandleFunc("GET /articles", articleH.List)
	mux.HandleFunc("GET /articles/{id}", articleH.Detail)

	// Admin pages (protected)
	mux.HandleFunc("GET /admin", auth(adminH.Page))

	// API endpoints (protected)
	mux.HandleFunc("GET /api/articles/{id}", auth(articleH.GetArticle))
	mux.HandleFunc("POST /api/articles", auth(articleH.Create))
	mux.HandleFunc("PUT /api/articles/{id}", auth(articleH.Update))
	mux.HandleFunc("DELETE /api/articles/{id}", auth(articleH.Delete))

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
