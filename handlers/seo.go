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

// --- Canonical URL ---

// canonicalURL builds the full canonical URL from the request.
func canonicalURL(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, path)
}

// --- Markdown stripping (for plain-text descriptions) ---

var (
	reHeading  = regexp.MustCompile(`^#{1,6}\s+`)
	reLink     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reImage    = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reCode     = regexp.MustCompile("`{1,3}[^`]*`{1,3}")
	reBold     = regexp.MustCompile(`\*{1,2}([^*]+)\*{1,2}`)
	reListItem = regexp.MustCompile(`^[\s]*[-*+]\s+`)
	reHTML     = regexp.MustCompile(`<[^>]*>`)
)

// stripMarkdown removes common markdown syntax for use as plain-text description.
func stripMarkdown(s string) string {
	s = reImage.ReplaceAllString(s, "")
	s = reLink.ReplaceAllString(s, "$1")
	s = reCode.ReplaceAllString(s, "")
	s = reBold.ReplaceAllString(s, "$1")
	s = reHeading.ReplaceAllString(s, "")
	s = reListItem.ReplaceAllString(s, "")
	s = reHTML.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Collapse multiple spaces
	spaceRE := regexp.MustCompile(`\s+`)
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// truncate truncates a string to maxLen runes, appending "…" if needed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// --- Structured Data (JSON-LD) ---

// homeStructuredData returns JSON-LD for the homepage.
func homeStructuredData(canonical string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebSite","name":"AI 智能文章站","description":"一个带智能问答机器人的文章网站，基于 AI 大模型为你解答文章中的任何问题。","url":"%s"}</script>`,
		canonical,
	))
}

// articleListStructuredData returns JSON-LD for the article list page.
func articleListStructuredData(canonical string, articles []models.Article) template.HTML {
	// canonical is e.g. "http://host/articles" — extract base for article URLs
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

// articleDetailStructuredData returns JSON-LD for an article detail page.
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
