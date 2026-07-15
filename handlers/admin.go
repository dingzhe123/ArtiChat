package handlers

import (
	"html/template"
	"net/http"
)

type AdminHandler struct {
	Tmpl *template.Template
}

func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "管理后台 — AI 智能文章站",
	}
	if err := h.Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
