package handlers

import (
	"html/template"
	"net/http"
)

// HomeHandler 处理主页请求（纯静态，不查询数据库）。
type HomeHandler struct {
	Tmpl *template.Template
}

// ServeHTTP 渲染主页 — GET /。
func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	canonical := canonicalURL(r, "/")
	data := map[string]interface{}{
		"Title":          "AI 智能文章站 — 探索知识，与 AI 对话",
		"Description":    "一个带智能问答机器人的文章网站，基于 AI 大模型为你解答文章中的任何问题。",
		"CanonicalURL":   canonical,
		"OGType":         "website",
		"StructuredData": homeStructuredData(canonical),
	}
	if err := h.Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
