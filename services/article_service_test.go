package services

import (
	"os"
	"testing"

	"ai-article-site/models"
)

func TestArticleService_CRUD(t *testing.T) {
	dbPath := "test_articles.db"
	defer os.Remove(dbPath) // clean up after test

	svc, err := NewArticleService(dbPath)
	if err != nil {
		t.Fatalf("NewArticleService: %v", err)
	}
	defer svc.Close()

	// --- Create ---
	a := &models.Article{
		Title:   "Hello World",
		Content: "This is a test article.",
		Author:  "Alice",
		Tags:    []string{"go", "test"},
	}
	id, err := svc.Create(a)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id != 1 {
		t.Fatalf("expected id=1, got %d", id)
	}

	// --- GetByID ---
	got, err := svc.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Hello World" {
		t.Fatalf("expected title 'Hello World', got '%s'", got.Title)
	}
	if len(got.Tags) != 2 || got.Tags[1] != "test" {
		t.Fatalf("unexpected tags: %v", got.Tags)
	}

	// --- List ---
	articles, err := svc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}

	// --- Update ---
	a.ID = id
	a.Title = "Updated Title"
	if err := svc.Update(a); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = svc.GetByID(id)
	if got.Title != "Updated Title" {
		t.Fatalf("expected 'Updated Title', got '%s'", got.Title)
	}

	// --- Delete ---
	if err := svc.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	articles, _ = svc.List()
	if len(articles) != 0 {
		t.Fatalf("expected 0 articles after delete, got %d", len(articles))
	}

	t.Log("All CRUD tests passed!")
}
