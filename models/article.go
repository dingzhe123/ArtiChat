package models

import "time"

// Article represents a published article.
type Article struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"` // Markdown
	Author    string    `json:"author"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ArticleChunk is a text segment with its vector embedding, used for RAG retrieval.
type ArticleChunk struct {
	ID        int64     `json:"id"`
	ArticleID int64     `json:"article_id"`
	ChunkIdx  int       `json:"chunk_idx"`
	Content   string    `json:"content"`
	Embedding []float64 `json:"embedding"`
}
