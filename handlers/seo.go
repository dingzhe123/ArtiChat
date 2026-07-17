package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ai-article-site/models"
)

// canonicalURL 根据请求构造完整的规范链接。
func canonicalURL(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, path)
}

// 用于清洗 Markdown 格式的正则表达式。
var (
	reHeading  = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reLink     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reImage    = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reCode     = regexp.MustCompile("`{1,3}[^`]*`{1,3}")
	reBold     = regexp.MustCompile(`\*{1,2}([^*]+)\*{1,2}`)
	reListItem = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`)
	reHTML     = regexp.MustCompile(`<[^>]*>`)
)

// StripMarkdown 移除常见 Markdown 语法，返回纯文本用于描述。
func StripMarkdown(s string) string {
	s = reImage.ReplaceAllString(s, "")
	s = reLink.ReplaceAllString(s, "$1")
	s = reCode.ReplaceAllString(s, "")
	s = reBold.ReplaceAllString(s, "$1")
	s = reHeading.ReplaceAllString(s, "")
	s = reListItem.ReplaceAllString(s, "")
	s = reHTML.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	spaceRE := regexp.MustCompile(`\s+`)
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// Truncate 将字符串截断到 maxLen 个字符，超出部分用 "…" 代替。
func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// homeStructuredData 返回首页的 JSON-LD 结构化数据。
func homeStructuredData(canonical string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebSite","name":"AI 智能文章站","description":"一个带智能问答机器人的文章网站，基于 AI 大模型为你解答文章中的任何问题。","url":"%s"}</script>`,
		canonical,
	))
}

// articleListStructuredData 返回文章列表页的 JSON-LD。
func articleListStructuredData(canonical string, articles []models.Article) template.HTML {
	// canonical 格式为 "http://host/articles"，提取基础路径拼文章链接
	base := strings.TrimSuffix(canonical, "/articles")
	items := make([]string, 0, len(articles))
	for i, a := range articles {
		items = append(items, fmt.Sprintf(
			`{"@type":"ListItem","position":%d,"url":"%s/articles/%d","name":"%s"}`,
			i+1, base, a.ID, template.JSEscapeString(a.Title),
		))
	}
	return template.HTML(fmt.Sprintf(
		`<script type="application/ld+json">{"@context":"https://schema.org","@type":"CollectionPage","name":"文章列表","description":"浏览所有已发布的文章。","url":"%s","mainEntity":{"@type":"ItemList","itemListElement":[%s]}}</script>`,
		canonical, strings.Join(items, ","),
	))
}

// articleDetailStructuredData 返回文章详情页的 JSON-LD。
func articleDetailStructuredData(canonical string, a *models.Article, desc string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<script type="application/ld+json">{"@context":"https://schema.org","@type":"Article","headline":"%s","description":"%s","author":{"@type":"Person","name":"%s"},"datePublished":"%s","dateModified":"%s","url":"%s"}</script>`,
		template.JSEscapeString(a.Title),
		template.JSEscapeString(desc),
		template.JSEscapeString(a.Author),
		a.CreatedAt.Format(time.RFC3339),
		a.UpdatedAt.Format(time.RFC3339),
		canonical,
	))
}
