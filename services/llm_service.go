package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ChatMessage 是 OpenAI 格式的对话消息。
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

// LLMClient 封装 OpenAI 兼容 API 的调用。
type LLMClient struct {
	apiKey     string
	baseURL    string
	model      string
	embedModel string
	client     *http.Client // 长超时（Chat）
	embClient  *http.Client // 短超时（Embedding）
}

// NewLLMClient 创建大模型 API 客户端。
func NewLLMClient(apiKey, baseURL, model, embedModel string) *LLMClient {
	return &LLMClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		embedModel: embedModel,
		client:     &http.Client{Timeout: 120 * time.Second},
		embClient:  &http.Client{Timeout: 8 * time.Second},
	}
}

// Chat 发送对话消息，返回助理的回复（非流式）。
func (c *LLMClient) Chat(messages []ChatMessage) (string, error) {
	body := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.7,
		Stream:      false,
	}

	resp, err := c.doJSON(c.client, "POST", c.baseURL+"/chat/completions", body)
	if err != nil {
		return "", err
	}

	var cr chatResponse
	if err := json.Unmarshal(resp, &cr); err != nil {
		return "", fmt.Errorf("解析 Chat 响应: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("LLM 错误: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("LLM 未返回任何选项")
	}
	return cr.Choices[0].Message.Content, nil
}

// ChatStream 发送流式对话请求，将 SSE 事件直接写入 w。
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
		return fmt.Errorf("流式请求: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LLM 流式错误 %d: %s", resp.StatusCode, string(b))
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("复制流: %w", err)
	}
	return nil
}

// Embed 返回文本的 Embedding 向量，使用短超时以便快速回退。
func (c *LLMClient) Embed(text string) ([]float64, error) {
	body := embedRequest{
		Model: c.embedModel,
		Input: text,
	}

	resp, err := c.doJSON(c.embClient, "POST", c.baseURL+"/embeddings", body)
	if err != nil {
		return nil, err
	}

	var er embedResponse
	if err := json.Unmarshal(resp, &er); err != nil {
		return nil, fmt.Errorf("解析 Embedding 响应: %w", err)
	}
	if er.Error != nil {
		return nil, fmt.Errorf("Embedding 错误: %s", er.Error.Message)
	}
	if len(er.Data) == 0 {
		return nil, fmt.Errorf("未返回 Embedding 数据")
	}
	return er.Data[0].Embedding, nil
}

// doJSON 使用默认客户端发送 JSON 请求。
func (c *LLMClient) doJSON(client *http.Client, method, url string, body interface{}) ([]byte, error) {
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

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API 状态 %d: %s", resp.StatusCode, string(b))
	}
	return b, nil
}
