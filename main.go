package main

import (
	"html/template"
	"log"
	"net/http"

	"ai-article-site/config"
	"ai-article-site/handlers"
	"ai-article-site/middleware"
	"ai-article-site/services"
)

// templateFuncs 向模板暴露的辅助函数，用于清洗 Markdown 和截断文本。
var templateFuncs = template.FuncMap{
	"StripMarkdown": handlers.StripMarkdown,
	"Truncate":      handlers.Truncate,
}

// parseSet 将 base.html 与指定的内容模板文件解析为一个模板集。
// 每个模板集拥有独立的 "content" 定义，避免 ParseGlob 带来的命名冲突。
func parseSet(contentFiles ...string) *template.Template {
	paths := append([]string{"templates/base.html"}, contentFiles...)
	return template.Must(template.New("base.html").Funcs(templateFuncs).ParseFiles(paths...))
}

func main() {
	cfg := config.Load()

	// 初始化各服务
	articleSvc, err := services.NewArticleService(cfg.DBPath)
	if err != nil {
		log.Fatalf("初始化文章服务失败: %v", err)
	}
	defer articleSvc.Close()

	// 大模型客户端
	llmClient := services.NewLLMClient(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.LLMModel, cfg.EmbeddingModel)

	// RAG 检索服务
	ragSvc, err := services.NewRAGService(articleSvc.DB(), llmClient)
	if err != nil {
		log.Fatalf("初始化 RAG 服务失败: %v", err)
	}

	// 为每个处理器构建独立的模板集，确保 {{block "content"}} 解析到正确的页面。
	homeTmpl := parseSet("templates/home.html")
	articlesListTmpl := parseSet("templates/article_list.html")
	articlesDetailTmpl := parseSet("templates/article_detail.html")
	adminTmpl := parseSet("templates/admin.html")

	// 各处理器
	homeH := &handlers.HomeHandler{Tmpl: homeTmpl}
	articleH := &handlers.ArticleHandler{
		ListTmpl:   articlesListTmpl,
		DetailTmpl: articlesDetailTmpl,
		Service:    articleSvc,
		RAG:        ragSvc,
	}
	adminH := &handlers.AdminHandler{Tmpl: adminTmpl, Service: articleSvc}
	chatH := &handlers.ChatHandler{
		RAG:            ragSvc,
		LLM:            llmClient,
		ArticleService: articleSvc,
	}

	// 管理后台 Basic Auth 保护
	auth := handlers.BasicAuth(cfg.AdminUser, cfg.AdminPass)

	// 路由注册（Go 1.22+ 增强路由，支持 HTTP 方法匹配）
	mux := http.NewServeMux()

	// 公开页面
	mux.HandleFunc("GET /", homeH.ServeHTTP)
	mux.HandleFunc("GET /articles", articleH.List)
	mux.HandleFunc("GET /articles/{id}", articleH.Detail)

	// 管理后台（需认证）
	mux.HandleFunc("GET /admin", auth(adminH.Page))

	// 文章 CRUD API（需认证）
	mux.HandleFunc("GET /api/articles/{id}", auth(articleH.GetArticle))
	mux.HandleFunc("POST /api/articles", auth(articleH.Create))
	mux.HandleFunc("PUT /api/articles/{id}", auth(articleH.Update))
	mux.HandleFunc("DELETE /api/articles/{id}", auth(articleH.Delete))

	// 智能问答 API（公开）
	mux.HandleFunc("POST /api/chat", chatH.HandleChat)

	// 重建索引 API（需认证）
	mux.HandleFunc("POST /api/reindex", auth(chatH.HandleReindex))

	// 静态文件服务
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// 应用中间件: CORS → 日志 → 路由
	handler := middleware.Logger(middleware.CORS(mux))

	log.Printf("服务器已启动 http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("服务器异常退出: %v", err)
	}
}
