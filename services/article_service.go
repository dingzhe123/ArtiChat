package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ai-article-site/models"

	_ "modernc.org/sqlite"
)

// ArticleService 处理文章在 SQLite 中的持久化。
type ArticleService struct {
	db *sql.DB
}

// NewArticleService 打开（或创建）SQLite 数据库并确保 articles 表存在。
func NewArticleService(dbPath string) (*ArticleService, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}

	// 启用 WAL 模式提升并发读取性能
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("启用 WAL: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("创建表: %w", err)
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

// DB 返回底层数据库连接，供其他服务使用。
func (s *ArticleService) DB() *sql.DB {
	return s.db
}

// Close 关闭数据库连接。
func (s *ArticleService) Close() error {
	return s.db.Close()
}

// Create 插入新文章，返回自增 ID。
func (s *ArticleService) Create(a *models.Article) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = time.Now().UTC()
	}

	if a.Tags == nil {
		a.Tags = []string{}
	}
	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		return 0, fmt.Errorf("序列化标签: %w", err)
	}

	result, err := s.db.Exec(
		"INSERT INTO articles (title, content, author, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		a.Title, a.Content, a.Author, string(tagsJSON), now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("插入文章: %w", err)
	}
	return result.LastInsertId()
}

// GetByID 按主键查询单篇文章。
func (s *ArticleService) GetByID(id int64) (*models.Article, error) {
	row := s.db.QueryRow("SELECT id, title, content, author, tags, created_at, updated_at FROM articles WHERE id = ?", id)
	return scanArticle(row)
}

// List 返回所有文章，按创建时间降序排列。
func (s *ArticleService) List() ([]models.Article, error) {
	rows, err := s.db.Query("SELECT id, title, content, author, tags, created_at, updated_at FROM articles ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("查询文章列表: %w", err)
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

// ListPage 返回指定页的文章和文章总数，按创建时间降序排列。
// page 从 1 开始，perPage 为每页条数。
func (s *ArticleService) ListPage(page, perPage int) ([]models.Article, int, error) {
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计文章总数: %w", err)
	}

	rows, err := s.db.Query(
		"SELECT id, title, content, author, tags, created_at, updated_at FROM articles ORDER BY created_at DESC LIMIT ? OFFSET ?",
		perPage, (page-1)*perPage,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("查询文章列表: %w", err)
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		a, err := scanArticleFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		articles = append(articles, *a)
	}
	return articles, total, rows.Err()
}

// Update 修改已有文章，若 ID 不存在则返回错误。
func (s *ArticleService) Update(a *models.Article) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if a.Tags == nil {
		a.Tags = []string{}
	}
	tagsJSON, err := json.Marshal(a.Tags)
	if err != nil {
		return fmt.Errorf("序列化标签: %w", err)
	}

	result, err := s.db.Exec(
		"UPDATE articles SET title=?, content=?, author=?, tags=?, updated_at=? WHERE id=?",
		a.Title, a.Content, a.Author, string(tagsJSON), now, a.ID,
	)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("文章 %d 不存在", a.ID)
	}
	return nil
}

// Delete 按 ID 删除文章，若 ID 不存在则返回错误。
func (s *ArticleService) Delete(id int64) error {
	result, err := s.db.Exec("DELETE FROM articles WHERE id=?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("文章 %d 不存在", id)
	}
	return nil
}

// scanArticle 从 Scanner 读取一行并解析为 Article。
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
