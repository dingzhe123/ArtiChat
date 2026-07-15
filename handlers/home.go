package handlers

import (
	"html/template"
	"net/http"

	"ai-article-site/services"
)

// HomeHandler serves the homepage.
type HomeHandler struct {
	Tmpl    *template.Template
	Service *services.ArticleService
}

// ServeHTTP renders the home page — GET /.
func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	articles, _ := h.Service.List()
	// Limit to 5 newest
	recent := articles
	if len(recent) > 5 {
		recent = recent[:5]
	}

	canonical := canonicalURL(r, "/")
	data := map[string]interface{}{
		"Title":          "AI 智能文章站 — 探索知识，与 AI 对话",
		"Description":    "一个带智能问答机器人的文章网站，基于 AI 大模型为你解答文章中的任何问题。",
		"CanonicalURL":   canonical,
		"OGType":         "website",
		"StructuredData": homeStructuredData(canonical),
		"Articles":       recent,
		"HasArticles":    len(recent) > 0,
	}
	if err := h.Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
