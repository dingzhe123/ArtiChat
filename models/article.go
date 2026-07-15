package models

import "time"

// Article 表示一篇已发布的文章。
type Article struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"` // Markdown 格式
	Author    string    `json:"author"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ArticleChunk 是文章的文本片段及其向量嵌入，用于 RAG 检索。
type ArticleChunk struct {
	ID        int64     `json:"id"`
	ArticleID int64     `json:"article_id"`
	ChunkIdx  int       `json:"chunk_idx"`
	Content   string    `json:"content"`
	Embedding []float64 `json:"embedding"`
}
