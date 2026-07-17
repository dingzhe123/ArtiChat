package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-article-site/models"
	"ai-article-site/services"

	"github.com/yuin/goldmark"
)

const (
	maxArticleBodySize = 1 << 20 // 1 MB
	charsPerMinute     = 400     // 中文平均阅读速度（字/分钟）
)

// ArticleHandler 处理文章页面和 API 请求。
type ArticleHandler struct {
	ListTmpl   *template.Template
	DetailTmpl *template.Template
	Service    *services.ArticleService
	RAG        *services.RAGService // 可选：设置后自动在增删改时维护索引
}

// List 渲染文章列表页 — GET /articles。
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

// Detail 渲染单篇文章详情页 — GET /articles/{id}。
func (h *ArticleHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		serveNotFound(w)
		return
	}

	article, err := h.Service.GetByID(id)
	if err != nil {
		serveNotFound(w)
		return
	}

	// Markdown → HTML，并降级标题避免与页面 h1 重复
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(article.Content), &buf); err != nil {
		http.Error(w, "内容渲染失败", http.StatusInternalServerError)
		return
	}
	contentHTML := shiftHeadings(buf.String())

	// 纯文本描述 + 预估阅读时间
	desc := Truncate(StripMarkdown(article.Content), 160)
	charCount := len([]rune(article.Content))
	readMin := int(math.Ceil(float64(charCount) / float64(charsPerMinute)))
	if readMin < 1 {
		readMin = 1
	}

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
		"ContentHTML":      template.HTML(contentHTML),
		"ReadMin":          readMin,
	}
	if err := h.DetailTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// GetArticle 返回单篇文章的 JSON — GET /api/articles/{id}。
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

// Create 处理文章创建 — POST /api/articles。
func (h *ArticleHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxArticleBodySize)
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
		log.Printf("创建文章失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": "创建文章失败",
		})
		return
	}

	a.ID = id
	// 异步生成向量索引
	if h.RAG != nil {
		go func() { _ = h.RAG.IndexArticle(&a) }()
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"ok": true, "data": a})
}

// Update 处理文章更新 — PUT /api/articles/{id}。
func (h *ArticleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 ID",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxArticleBodySize)
	var a models.Article
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 JSON 格式",
		})
		return
	}

	a.ID = id
	if err := h.Service.Update(&a); err != nil {
		log.Printf("更新文章 %d 失败: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": "更新文章失败",
		})
		return
	}

	updated, _ := h.Service.GetByID(id)
	// 重新索引更新后的文章
	if h.RAG != nil && updated != nil {
		go func() { _ = h.RAG.IndexArticle(updated) }()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": updated})
}

// Delete 处理文章删除 — DELETE /api/articles/{id}。
func (h *ArticleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 ID",
		})
		return
	}

	if err := h.Service.Delete(id); err != nil {
		log.Printf("删除文章 %d 失败: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": "删除文章失败",
		})
		return
	}

	// 清理关联的向量片段
	if h.RAG != nil {
		_ = h.RAG.DeleteChunks(id)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// writeJSON 以 JSON 格式写入响应。
// writeJSON 以 JSON 格式写入响应。
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// serveNotFound 返回一个包含站点布局的 HTML 404 页面。
func serveNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>页面未找到 — AI 智能文章站</title>
<link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
<header class="site-header"><div class="container">
<a href="/" class="logo">AI 智能文章站</a>
<nav><a href="/">首页</a><a href="/articles">文章</a><a href="/admin">管理</a></nav>
</div></header>
<main class="site-main"><div class="container" style="text-align:center;padding:80px 0;">
<h1>404</h1>
<p>页面未找到，该文章可能已被删除或不存在。</p>
<p><a href="/" class="btn btn-primary">返回首页</a>
<a href="/articles" class="btn btn-secondary">浏览文章</a></p>
</div></main>
<footer class="site-footer"><div class="container"><p>AI 智能文章站</p></div></footer>
</body>
</html>`)
}

// shiftHeadings 将 Markdown 渲染出的 HTML 标题降一级（h1→h2, h2→h3…），
// 确保页面模板中的 <h1> 是唯一的，满足 SEO 最佳实践。
func shiftHeadings(html string) string {
	return strings.NewReplacer(
		"<h5>", "<h6>", "</h5>", "</h6>",
		"<h4>", "<h5>", "</h4>", "</h5>",
		"<h3>", "<h4>", "</h3>", "</h4>",
		"<h2>", "<h3>", "</h2>", "</h3>",
		"<h1>", "<h2>", "</h1>", "</h2>",
	).Replace(html)
}
