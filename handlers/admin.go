package handlers

import (
	"html/template"
	"net/http"

	"ai-article-site/services"
)

// AdminHandler handles the admin management page.
type AdminHandler struct {
	Tmpl    *template.Template
	Service *services.ArticleService
}

// Page renders the admin dashboard — GET /admin.
func (h *AdminHandler) Page(w http.ResponseWriter, r *http.Request) {
	articles, err := h.Service.List()
	if err != nil {
		http.Error(w, "加载文章失败", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":    "管理后台 — AI 智能文章站",
		"Articles": articles,
	}
	if err := h.Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
