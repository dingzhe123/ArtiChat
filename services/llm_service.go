package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// --- message types ---

// ChatMessage is a message in the OpenAI chat format.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// --- client ---

// LLMClient calls an OpenAI-compatible API.
type LLMClient struct {
	apiKey  string
	baseURL string
	model   string
	embedModel string
	client  *http.Client
}

// NewLLMClient creates a new LLM API client.
func NewLLMClient(apiKey, baseURL, model, embedModel string) *LLMClient {
	return &LLMClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		embedModel: embedModel,
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

// Chat sends a list of messages and returns the assistant reply (non-streaming).
func (c *LLMClient) Chat(messages []ChatMessage) (string, error) {
	body := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.7,
		Stream:      false,
	}

	resp, err := c.doJSON("POST", c.baseURL+"/chat/completions", body)
	if err != nil {
		return "", err
	}

	var cr chatResponse
	if err := json.Unmarshal(resp, &cr); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("LLM error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}
	return cr.Choices[0].Message.Content, nil
}

// ChatStream sends a streaming chat request and writes SSE events to w.
func (c *LLMClient) ChatStream(messages []ChatMessage, w http.ResponseWriter) error {
	body := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.7,
		Stream:      true,
	}

	reqBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(reqBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("chat stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LLM stream status %d: %s", resp.StatusCode, string(b))
	}

	// Copy SSE stream directly to client
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy stream: %w", err)
	}
	return nil
}

// Embed returns the embedding vector for the given text.
func (c *LLMClient) Embed(text string) ([]float64, error) {
	body := embedRequest{
		Model: c.embedModel,
		Input: text,
	}

	resp, err := c.doJSON("POST", c.baseURL+"/embeddings", body)
	if err != nil {
		return nil, err
	}

	var er embedResponse
	if err := json.Unmarshal(resp, &er); err != nil {
		return nil, fmt.Errorf("parse embed response: %w", err)
	}
	if er.Error != nil {
		return nil, fmt.Errorf("Embedding error: %s", er.Error.Message)
	}
	if len(er.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return er.Data[0].Embedding, nil
}

// --- helpers ---

func (c *LLMClient) doJSON(method, url string, body interface{}) ([]byte, error) {
	reqBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API status %d: %s", resp.StatusCode, string(b))
	}
	return b, nil
}
