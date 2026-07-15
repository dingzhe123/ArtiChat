package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ai-article-site/models"

	_ "modernc.org/sqlite"
)

// ArticleService handles article persistence in SQLite.
type ArticleService struct {
	db *sql.DB
}

// NewArticleService opens (or creates) the SQLite database and ensures the
// articles table exists.
func NewArticleService(dbPath string) (*ArticleService, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode for better concurrent reads.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &ArticleService{db: db}, nil
}

func createTables(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS articles (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		title      TEXT    NOT NULL,
		content    TEXT    NOT NULL DEFAULT '',
		author     TEXT    NOT NULL DEFAULT '',
		tags       TEXT    NOT NULL DEFAULT '[]',
		created_at TEXT    NOT NULL,
		updated_at TEXT    NOT NULL
	)`
	_, err := db.Exec(query)
	return err
}

// DB returns the underlying database connection (for use by other services).
func (s *ArticleService) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database connection.
func (s *ArticleService) Close() error {
	return s.db.Close()
}

// Create inserts a new article and returns its ID.
func (s *ArticleService) Create(a *models.Article) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = time.Now().UTC()
	}

	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		return 0, fmt.Errorf("marshal tags: %w", err)
	}

	result, err := s.db.Exec(
		"INSERT INTO articles (title, content, author, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		a.Title, a.Content, a.Author, string(tagsJSON), now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert article: %w", err)
	}
	return result.LastInsertId()
}

// GetByID retrieves a single article by primary key.
func (s *ArticleService) GetByID(id int64) (*models.Article, error) {
	row := s.db.QueryRow("SELECT id, title, content, author, tags, created_at, updated_at FROM articles WHERE id = ?", id)
	return scanArticle(row)
}

// List returns all articles ordered by creation time (newest first).
func (s *ArticleService) List() ([]models.Article, error) {
	rows, err := s.db.Query("SELECT id, title, content, author, tags, created_at, updated_at FROM articles ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query articles: %w", err)
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		a, err := scanArticleFromRows(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, rows.Err()
}

// Update modifies an existing article.
func (s *ArticleService) Update(a *models.Article) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	_, err = s.db.Exec(
		"UPDATE articles SET title=?, content=?, author=?, tags=?, updated_at=? WHERE id=?",
		a.Title, a.Content, a.Author, string(tagsJSON), now, a.ID,
	)
	return err
}

// Delete removes an article by ID.
func (s *ArticleService) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM articles WHERE id=?", id)
	return err
}

func scanArticle(scanner interface{ Scan(...interface{}) error }) (*models.Article, error) {
	var a models.Article
	var tagsJSON string
	var createdAt, updatedAt string

	err := scanner.Scan(&a.ID, &a.Title, &a.Content, &a.Author, &tagsJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(tagsJSON), &a.Tags)
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &a, nil
}

func scanArticleFromRows(rows *sql.Rows) (*models.Article, error) {
	return scanArticle(rows)
}
