package handlers

import (
	"html/template"
	"net/http"
)

type ArticleHandler struct {
	Tmpl *template.Template
}

func (h *ArticleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":       "文章列表 — AI 智能文章站",
		"Description": "浏览所有已发布的文章。",
	}
	if err := h.Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
