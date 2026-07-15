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
	"unicode/utf8"

	"ai-article-site/models"
)

// RAGService handles article chunking, embedding, and retrieval.
type RAGService struct {
	db    *sql.DB
	llm   *LLMClient
}

// NewRAGService creates a new RAG service. It ensures the chunks table exists.
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

// --- chunking ---

const maxChunkLen = 500

// chunkArticle splits article content into text chunks by paragraphs.
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
		// Split long paragraph by sentences
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

// --- indexing ---

// IndexArticle chunks an article, optionally embeds, and stores the chunks.
// It tolerates embedding failures — chunks are stored with empty embeddings
// and keyword search will still work.
func (s *RAGService) IndexArticle(article *models.Article) error {
	if err := s.DeleteChunks(article.ID); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	chunks := chunkArticle(article.Content)
	if len(chunks) == 0 {
		return nil
	}

	for idx, chunk := range chunks {
		var embJSON string
		// Try embedding; fall back to empty on any error
		embedding, err := s.llm.Embed(chunk)
		if err != nil {
			log.Printf("WARN: embed failed for article %d chunk %d (will use keyword search): %v",
				article.ID, idx, err)
			embJSON = "[]"
		} else {
			b, _ := json.Marshal(embedding)
			embJSON = string(b)
		}

		if _, err := s.db.Exec(
			"INSERT INTO article_chunks (article_id, chunk_idx, content, embedding) VALUES (?, ?, ?, ?)",
			article.ID, idx, chunk, embJSON,
		); err != nil {
			return fmt.Errorf("insert chunk: %w", err)
		}
	}
	return nil
}

// DeleteChunks removes all chunks for a given article.
func (s *RAGService) DeleteChunks(articleID int64) error {
	_, err := s.db.Exec("DELETE FROM article_chunks WHERE article_id = ?", articleID)
	return err
}

// --- retrieval ---

// SearchResult is a chunk with its similarity score (0.0–1.0).
type SearchResult struct {
	Chunk      models.ArticleChunk
	Similarity float64
}

// Search finds the top-K most relevant chunks for a query.
// Uses embedding-based cosine similarity if embeddings exist;
// falls back to keyword matching otherwise.
func (s *RAGService) Search(query string, topK int) ([]SearchResult, error) {
	// Try embedding-based search first
	results, err := s.embeddingSearch(query)
	if err == nil && len(results) > 0 {
		return topKResults(results, topK), nil
	}

	// Fallback to keyword search
	log.Printf("INFO: using keyword search (embedding search returned %d results, err=%v)", len(results), err)
	return s.keywordSearch(query, topK)
}

// embeddingSearch generates a query embedding and scores all chunks by cosine similarity.
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

// keywordSearch uses TF-like word overlap to score chunks.
func (s *RAGService) keywordSearch(query string, topK int) ([]SearchResult, error) {
	rows, err := s.db.Query("SELECT id, article_id, chunk_idx, content, embedding FROM article_chunks")
	if err != nil {
		return nil, fmt.Errorf("query chunks: %w", err)
	}
	defer rows.Close()

	queryWords := tokenize(query)

	var results []SearchResult
	for rows.Next() {
		var c models.ArticleChunk
		var embJSON string
		if err := rows.Scan(&c.ID, &c.ArticleID, &c.ChunkIdx, &c.Content, &embJSON); err != nil {
			continue
		}

		if len(queryWords) == 0 {
			continue
		}

		chunkLower := strings.ToLower(c.Content)
		hits := 0
		for _, w := range queryWords {
			if strings.Contains(chunkLower, w) {
				hits++
			}
		}
		score := float64(hits) / float64(len(queryWords))
		if score > 0 {
			results = append(results, SearchResult{Chunk: c, Similarity: score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topKResults(results, topK), nil
}

func topKResults(results []SearchResult, topK int) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	if topK > len(results) {
		topK = len(results)
	}
	return results[:topK]
}

// --- tokenization ---

// tokenize splits text into search tokens.
// For Chinese, each character is a token. For English, words are tokens.
func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else if unicode.Is(unicode.Han, r) {
			// Flush current English word
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	// Deduplicate and filter short tokens
	seen := make(map[string]bool)
	var out []string
	for _, t := range tokens {
		if utf8.RuneCountInString(t) < 1 || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// ReindexAll clears all chunks and re-indexes all articles.
func (s *RAGService) ReindexAll(as *ArticleService) error {
	if _, err := s.db.Exec("DELETE FROM article_chunks"); err != nil {
		return fmt.Errorf("clear chunks: %w", err)
	}

	articles, err := as.List()
	if err != nil {
		return fmt.Errorf("list articles: %w", err)
	}

	log.Printf("Reindexing %d articles...", len(articles))
	for i := range articles {
		log.Printf("  [%d/%d] Indexing article %d: %s", i+1, len(articles), articles[i].ID, articles[i].Title)
		if err := s.IndexArticle(&articles[i]); err != nil {
			return fmt.Errorf("index article %d: %w", articles[i].ID, err)
		}
	}
	log.Printf("Reindex complete — %d articles indexed", len(articles))
	return nil
}

// --- similarity ---

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
