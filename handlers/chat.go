package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"ai-article-site/services"
)

const (
	maxChatBodySize = 32 << 10 // 32 KB
	maxQuestionLen  = 2000     // 字符
	maxContextChunks = 5
	// minSourceScore 是片段进入上下文和来源列表的最低相关性得分，
	// 过滤掉关键词检索中仅有零星字符重叠的无关片段。
	minSourceScore = 0.15
)

// ChatHandler 处理智能问答 API。
type ChatHandler struct {
	RAG            *services.RAGService
	LLM            *services.LLMClient
	ArticleService *services.ArticleService
}

// ChatRequest 是聊天请求的 JSON 结构。
type ChatRequest struct {
	Question string `json:"question"`
	Stream   bool   `json:"stream"`
}

// ChatResponse 是聊天响应的 JSON 结构。
type ChatResponse struct {
	Answer  string       `json:"answer"`
	Sources []ChatSource `json:"sources,omitempty"`
}

// ChatSource 是回答所引用的文章片段。
type ChatSource struct {
	ArticleID int64   `json:"article_id"`
	Content   string  `json:"content"`
	Score     float64 `json:"score"`
}

// HandleChat 处理问答请求 — POST /api/chat。
func (h *ChatHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "无效的 JSON 格式",
		})
		return
	}

	question := strings.TrimSpace(req.Question)
	if question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "问题不能为空",
		})
		return
	}
	if len([]rune(question)) > maxQuestionLen {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": fmt.Sprintf("问题长度不能超过 %d 个字符", maxQuestionLen),
		})
		return
	}

	// 1. RAG 检索相关文章片段
	results, err := h.RAG.Search(question, maxContextChunks)
	if err != nil {
		log.Printf("RAG 检索失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": "检索失败，请稍后重试",
		})
		return
	}

	// 2. 构建上下文和来源列表（过滤低相关性片段）
	var contextBuilder strings.Builder
	var sources []ChatSource
	for _, r := range results {
		if r.Similarity < minSourceScore {
			continue
		}
		contextBuilder.WriteString(r.Chunk.Content)
		contextBuilder.WriteString("\n\n")
		sources = append(sources, ChatSource{
			ArticleID: r.Chunk.ArticleID,
			Content:   Truncate(r.Chunk.Content, 200),
			Score:     r.Similarity,
		})
	}
	context := contextBuilder.String()

	// 3. 组装 Prompt
	systemPrompt := `你是一个知识助手，请根据以下文章内容回答用户的问题。

## 相关文章片段

` + context + `
## 回答要求

- 请基于上述文章片段回答问题
- 如果文章中没有相关信息，请如实告知用户
- 回答要简洁准确，条理清晰`

	messages := []services.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: question},
	}

	// 4. 流式或非流式调用
	if req.Stream {
		if err := h.LLM.ChatStream(messages, w); err != nil {
			log.Printf("LLM 流式调用失败: %v", err)
		}
		return
	}

	answer, err := h.LLM.Chat(messages)
	if err != nil {
		log.Printf("LLM 调用失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": "AI 服务暂时不可用，请稍后重试",
		})
		return
	}

	writeJSON(w, http.StatusOK, ChatResponse{
		Answer:  answer,
		Sources: sources,
	})
}

// HandleReindex 重建所有文章的向量索引 — POST /api/reindex。
func (h *ChatHandler) HandleReindex(w http.ResponseWriter, r *http.Request) {
	if h.ArticleService == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": "服务未配置",
		})
		return
	}

	stats, err := h.RAG.ReindexAll(h.ArticleService)
	if err != nil {
		log.Printf("重建索引失败: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok": false, "error": "重建索引失败，请稍后重试",
		})
		return
	}

	// 如实反映向量生成结果，避免 embedding 全部失败时误报成功
	var message string
	switch {
	case stats.Chunks == 0:
		message = "索引已重建，但没有可索引的内容"
	case stats.Fallback == 0:
		message = fmt.Sprintf("向量索引重建完成（%d 篇文章，%d 个片段）", stats.Articles, stats.Chunks)
	case stats.Embedded == 0:
		message = fmt.Sprintf("索引已重建，但全部 %d 个片段的向量生成均失败，将仅使用关键词检索（请检查 Embedding 配置）", stats.Chunks)
	default:
		message = fmt.Sprintf("索引已重建：%d/%d 个片段向量生成成功，其余回退关键词检索", stats.Embedded, stats.Chunks)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": message,
		"stats":   stats,
	})
}
