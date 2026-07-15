package handlers

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"ai-article-site/models"
	"ai-article-site/services"

	"github.com/yuin/goldmark"
)

// ArticleHandler handles article pages and API.
type ArticleHandler struct {
	ListTmpl   *template.Template
	DetailTmpl *template.Template
	Service    *services.ArticleService
}

// List renders the article list page — GET /articles.
func (h *ArticleHandler) List(w http.ResponseWriter, r *http.Request) {
	articles, err := h.Service.List()
	if err != nil {
		http.Error(w, "加载文章失败", http.StatusInternalServerError)
		return
	}

	canonical := canonicalURL(r, "/articles")
	data := map[string]interface{}{
		"Title":          "文章列表 — AI 智能文章站",
		"Description":    "浏览所有已发布的文章。",
		"CanonicalURL":   canonical,
		"OGType":         "website",
		"StructuredData": articleListStructuredData(canonical, articles),
		"Articles":       articles,
	}
	if err := h.ListTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Detail renders a single article page — GET /articles/{id}.
func (h *ArticleHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	article, err := h.Service.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Render Markdown → HTML
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(article.Content), &buf); err != nil {
		http.Error(w, "内容渲染失败", http.StatusInternalServerError)
		return
	}

	// Plain-text description from markdown
	desc := truncate(stripMarkdown(article.Content), 160)

	canonical := canonicalURL(r, "/articles/"+strconv.FormatInt(article.ID, 10))
	data := map[string]interface{}{
		"Title":            article.Title + " — AI 智能文章站",
		"Description":      desc,
		"CanonicalURL":     canonical,
		"OGType":           "article",
		"ArticleAuthor":    article.Author,
		"ArticlePublished": article.CreatedAt.Format(time.RFC3339),
		"ArticleModified":  article.UpdatedAt.Format(time.RFC3339),
		"StructuredData":   articleDetailStructuredData(canonical, article, desc),
		"Article":          article,
		"ContentHTML":      template.HTML(buf.String()),
	}
	if err := h.DetailTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// GetArticle returns a single article as JSON — GET /api/articles/{id}.
func (h *ArticleHandler) GetArticle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 ID",
		})
		return
	}

	article, err := h.Service.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok": false, "error": "文章不存在",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": article})
}

// Create handles article creation — POST /api/articles.
func (h *ArticleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var a models.Article
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 JSON 格式",
		})
		return
	}
	if a.Title == "" || a.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "标题和内容不能为空",
		})
		return
	}

	id, err := h.Service.Create(&a)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	a.ID = id
	writeJSON(w, http.StatusCreated, map[string]interface{}{"ok": true, "data": a})
}

// Update handles article update — PUT /api/articles/{id}.
func (h *ArticleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 ID",
		})
		return
	}

	var a models.Article
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 JSON 格式",
		})
		return
	}

	a.ID = id
	if err := h.Service.Update(&a); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	updated, _ := h.Service.GetByID(id)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": updated})
}

// Delete handles article deletion — DELETE /api/articles/{id}.
func (h *ArticleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 ID",
		})
		return
	}

	if err := h.Service.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
