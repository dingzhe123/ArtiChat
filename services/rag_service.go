package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"unicode"

	"ai-article-site/models"
)

// RAGService 处理文章分块、向量嵌入与检索。
type RAGService struct {
	db  *sql.DB
	llm *LLMClient
}

// NewRAGService 创建 RAG 服务并确保分块表存在。
func NewRAGService(db *sql.DB, llm *LLMClient) (*RAGService, error) {
	svc := &RAGService{db: db, llm: llm}
	if err := svc.createChunksTable(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *RAGService) createChunksTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS article_chunks (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			article_id INTEGER NOT NULL,
			chunk_idx  INTEGER NOT NULL,
			content    TEXT    NOT NULL,
			embedding  TEXT    NOT NULL DEFAULT '[]',
			FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE
		)
	`)
	return err
}

// 每个文本块的最大字符数。
const maxChunkLen = 500

// chunkArticle 将文章内容按段落切分为文本块。
func chunkArticle(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	paragraphs := strings.Split(content, "\n\n")
	var chunks []string

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len([]rune(p)) <= maxChunkLen {
			chunks = append(chunks, p)
			continue
		}
		// 长段落按句子拆分
		sentences := splitSentences(p)
		current := ""
		for _, s := range sentences {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if len([]rune(current))+len([]rune(s)) <= maxChunkLen {
				if current != "" {
					current += "\n"
				}
				current += s
			} else {
				if len([]rune(current)) >= 50 {
					chunks = append(chunks, current)
				}
				current = s
			}
		}
		if len([]rune(current)) >= 50 {
			chunks = append(chunks, current)
		}
	}
	return chunks
}

// splitSentences 按中英文句末标点拆分句子。
func splitSentences(text string) []string {
	var result []string
	current := ""
	for _, r := range text {
		current += string(r)
		if r == '。' || r == '！' || r == '？' || r == '.' || r == '!' || r == '?' || r == '\n' {
			result = append(result, strings.TrimSpace(current))
			current = ""
		}
	}
	if strings.TrimSpace(current) != "" {
		result = append(result, strings.TrimSpace(current))
	}
	return result
}

// IndexArticle 为文章生成文本块并存入数据库。
// Embedding 失败时静默回退，块仍会被存储并支持关键词检索。
func (s *RAGService) IndexArticle(article *models.Article) error {
	if err := s.DeleteChunks(article.ID); err != nil {
		return fmt.Errorf("清除旧块: %w", err)
	}

	chunks := chunkArticle(article.Content)
	if len(chunks) == 0 {
		return nil
	}

	for idx, chunk := range chunks {
		var embJSON string
		embedding, err := s.llm.Embed(chunk)
		if err != nil {
			log.Printf("Embedding 失败（文章 %d 块 %d，将使用关键词检索）: %v", article.ID, idx, err)
			embJSON = "[]"
		} else {
			b, _ := json.Marshal(embedding)
			embJSON = string(b)
		}

		if _, err := s.db.Exec(
			"INSERT INTO article_chunks (article_id, chunk_idx, content, embedding) VALUES (?, ?, ?, ?)",
			article.ID, idx, chunk, embJSON,
		); err != nil {
			return fmt.Errorf("插入块: %w", err)
		}
	}
	return nil
}

// DeleteChunks 删除指定文章的所有文本块。
func (s *RAGService) DeleteChunks(articleID int64) error {
	_, err := s.db.Exec("DELETE FROM article_chunks WHERE article_id = ?", articleID)
	return err
}

// SearchResult 是文本块及其相似度得分（0.0–1.0）。
type SearchResult struct {
	Chunk      models.ArticleChunk
	Similarity float64
}

// Search 查找与查询最相关的 topK 个文本块。优先使用向量检索，失败时回退到关键词匹配。
func (s *RAGService) Search(query string, topK int) ([]SearchResult, error) {
	results, err := s.embeddingSearch(query)
	if err == nil && len(results) > 0 {
		return topKResults(results, topK), nil
	}

	log.Printf("使用关键词检索（向量检索返回 %d 条结果，错误: %v）", len(results), err)
	return s.keywordSearch(query, topK)
}

// embeddingSearch 通过余弦相似度进行向量检索。
func (s *RAGService) embeddingSearch(query string) ([]SearchResult, error) {
	queryVec, err := s.llm.Embed(query)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query("SELECT id, article_id, chunk_idx, content, embedding FROM article_chunks WHERE embedding != '[]'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var c models.ArticleChunk
		var embJSON string
		if err := rows.Scan(&c.ID, &c.ArticleID, &c.ChunkIdx, &c.Content, &embJSON); err != nil {
			continue
		}
		var emb []float64
		if err := json.Unmarshal([]byte(embJSON), &emb); err != nil {
			continue
		}
		c.Embedding = emb
		sim := cosineSimilarity(queryVec, emb)
		results = append(results, SearchResult{Chunk: c, Similarity: sim})
	}
	return results, rows.Err()
}

// keywordSearch 使用混合分词 + 最长公共子串进行关键词匹配。
func (s *RAGService) keywordSearch(query string, topK int) ([]SearchResult, error) {
	rows, err := s.db.Query("SELECT id, article_id, chunk_idx, content, embedding FROM article_chunks")
	if err != nil {
		return nil, fmt.Errorf("查询块: %w", err)
	}
	defer rows.Close()

	queryTokens := tokenizeHybrid(query)
	if len(queryTokens) == 0 {
		return nil, nil
	}
	querySet := toSet(queryTokens)
	queryLower := strings.ToLower(query)

	var results []SearchResult
	for rows.Next() {
		var c models.ArticleChunk
		var embJSON string
		if err := rows.Scan(&c.ID, &c.ArticleID, &c.ChunkIdx, &c.Content, &embJSON); err != nil {
			continue
		}

		chunkTokens := tokenizeHybrid(c.Content)
		chunkSet := toSet(chunkTokens)
		chunkLower := strings.ToLower(c.Content)

		// 1. 查询覆盖率：交集 / 查询词数
		intersection := 0
		for t := range querySet {
			if chunkSet[t] {
				intersection++
			}
		}
		score := float64(intersection) / float64(len(querySet))

		// 2. 最长公共子串加分
		lcsLen := longestCommonSubstring(queryLower, chunkLower)
		if lcsLen >= 6 {
			score += 0.4
		} else if lcsLen >= 4 {
			score += 0.25
		} else if lcsLen >= 3 {
			score += 0.1
		}

		if score > 0.01 {
			results = append(results, SearchResult{Chunk: c, Similarity: score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topKResults(results, topK), nil
}

// longestCommonSubstring 返回两个字符串的最长公共子串长度（按 rune 计算）。
func longestCommonSubstring(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	prev := make([]int, len(rb)+1)
	maxLen := 0
	for i := 1; i <= len(ra); i++ {
		curr := make([]int, len(rb)+1)
		for j := 1; j <= len(rb); j++ {
			if ra[i-1] == rb[j-1] {
				curr[j] = prev[j-1] + 1
				if curr[j] > maxLen {
					maxLen = curr[j]
				}
			}
		}
		prev = curr
	}
	return maxLen
}

// topKResults 按相似度降序排列并返回前 K 条。
func topKResults(results []SearchResult, topK int) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	if topK > len(results) {
		topK = len(results)
	}
	return results[:topK]
}

func toSet(tokens []string) map[string]bool {
	s := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		s[t] = true
	}
	return s
}

// 常见中文停用字符和词，分词时过滤。
var stopChars = map[string]bool{
	"的": true, "了": true, "是": true, "在": true, "和": true,
	"与": true, "或": true, "也": true, "都": true, "就": true,
	"而": true, "及": true, "且": true, "但": true, "所": true,
	"为": true, "以": true, "之": true, "其": true, "这": true,
	"那": true, "等": true, "被": true, "把": true, "从": true,
	"到": true, "对": true, "向": true, "让": true, "用": true,
	"还": true, "要": true, "会": true, "能": true, "可": true,
	"很": true, "着": true, "过": true, "地": true, "得": true,
	"吗": true, "呢": true, "吧": true, "啊": true, "嗯": true,
	"么": true, "怎": true, "哪": true, "什": true, "谁": true,
	"有": true, "不": true, "没": true, "个": true, "一": true,
	"来": true, "去": true, "说": true, "看": true, "想": true,
	"做": true, "上": true, "下": true, "中": true,
	"可以": true, "什么": true, "怎么": true, "为什么": true,
	"一个": true, "这个": true, "那个": true, "哪个": true,
	"一些": true, "一种": true, "一下": true, "自己": true,
	"知道": true, "没有": true, "他们": true, "我们": true,
	"如果": true, "因为": true, "所以": true,
	"但是": true, "然后": true, "不过": true, "还是": true,
	"已经": true, "可能": true, "应该": true, "需要": true,
	"这些": true, "那些": true, "如何": true,
	"对于": true, "关于": true, "以及": true, "并且": true,
}

// tokenizeHybrid 生成混合分词：中文 unigram + bigram，英文单词。
func tokenizeHybrid(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	var hanBuf []rune

	flushEng := func() {
		if current.Len() > 0 {
			w := current.String()
			if len(w) >= 2 && !stopChars[w] {
				tokens = append(tokens, w)
			}
			current.Reset()
		}
	}
	flushHan := func() {
		for _, r := range hanBuf {
			c := string(r)
			if !stopChars[c] {
				tokens = append(tokens, c) // unigram
			}
		}
		if len(hanBuf) >= 2 {
			for i := 0; i < len(hanBuf)-1; i++ {
				bigram := string(hanBuf[i]) + string(hanBuf[i+1])
				c1, c2 := string(hanBuf[i]), string(hanBuf[i+1])
				if stopChars[c1] && stopChars[c2] {
					continue
				}
				if !stopChars[bigram] {
					tokens = append(tokens, bigram) // bigram
				}
			}
		}
		hanBuf = nil
	}

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			flushEng()
			hanBuf = append(hanBuf, r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			flushHan()
			current.WriteRune(r)
		} else {
			flushHan()
			flushEng()
		}
	}
	flushHan()
	flushEng()

	// 去重
	seen := make(map[string]bool)
	var out []string
	for _, t := range tokens {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// ReindexAll 清空所有块并重新索引全部文章。
func (s *RAGService) ReindexAll(as *ArticleService) error {
	if _, err := s.db.Exec("DELETE FROM article_chunks"); err != nil {
		return fmt.Errorf("清空块: %w", err)
	}

	articles, err := as.List()
	if err != nil {
		return fmt.Errorf("列出文章: %w", err)
	}

	log.Printf("正在重建索引（共 %d 篇文章）...", len(articles))
	for i := range articles {
		log.Printf("  [%d/%d] 索引文章 %d: %s", i+1, len(articles), articles[i].ID, articles[i].Title)
		if err := s.IndexArticle(&articles[i]); err != nil {
			return fmt.Errorf("索引文章 %d: %w", articles[i].ID, err)
		}
	}
	log.Printf("索引重建完成 — 共 %d 篇文章", len(articles))
	return nil
}

// cosineSimilarity 计算两个向量的余弦相似度。
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
