package handlers

import (
	"fmt"
	"html/template"
	"net/http"

	"ai-article-site/services"
)

// AdminHandler 处理管理后台页面。
type AdminHandler struct {
	Tmpl    *template.Template
	Service *services.ArticleService
}

// Page 渲染管理后台 — GET /admin。
func (h *AdminHandler) Page(w http.ResponseWriter, r *http.Request) {
	articles, err := h.Service.List()
	if err != nil {
		http.Error(w, "加载文章失败", http.StatusInternalServerError)
		return
	}

	canonical := canonicalURL(r, "/admin")
	data := map[string]interface{}{
		"Title":        "管理后台 — AI 智能文章站",
		"Description":  "文章管理后台 — 创建、编辑、删除文章。",
		"CanonicalURL": canonical,
		"OGType":       "website",
		"StructuredData": template.HTML(fmt.Sprintf(
			`<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebPage","name":"管理后台","url":"%s"}</script>`,
			canonical,
		)),
		"Articles": articles,
	}
	if err := h.Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
