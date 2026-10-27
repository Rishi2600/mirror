// Package ai calls the LLM (Groq) to turn free text into typed records.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/events"
	"mirror/server/internal/profiles"
)

const groqURL = "https://api.groq.com/openai/v1/chat/completions"

// Groq talks to Groq's OpenAI-compatible chat API with strict JSON-schema
// output, so every reply matches the schema.
type Groq struct {
	apiKey string
	model  string
	url    string
	client *http.Client
	db     *gorm.DB
	logger *slog.Logger
}

func NewGroq(apiKey, model string, db *gorm.DB, logger *slog.Logger) *Groq {
	return &Groq{
		apiKey: apiKey,
		model:  model,
		url:    groqURL,
		client: &http.Client{Timeout: 50 * time.Second},
		db:     db,
		logger: logger,
	}
}

// ParseCV reads a CV's text into a profiles.CV.
func (g *Groq) ParseCV(ctx context.Context, text string) (profiles.CV, error) {
	var cv profiles.CV
	err := g.complete(ctx, "parse_cv", cvPrompt, text, "cv", cvSchema, &cv)
	return cv, err
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// complete sends one system and one user message and decodes the JSON
// reply into out. Every call is logged to events.ai_calls.
func (g *Groq) complete(ctx context.Context, purpose, system, user, schemaName, schema string, out any) error {
	body, err := json.Marshal(map[string]any{
		"model":            g.model,
		"temperature":      0,
		"reasoning_effort": "low",
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   schemaName,
				"strict": true,
				"schema": json.RawMessage(schema),
			},
		},
	})
	if err != nil {
		return err
	}

	start := time.Now()
	reply, err := g.post(ctx, body)
	call := events.AICall{
		Purpose:      purpose,
		Model:        g.model,
		LatencyMS:    int(time.Since(start).Milliseconds()),
		InputTokens:  reply.Usage.PromptTokens,
		OutputTokens: reply.Usage.CompletionTokens,
	}
	if err == nil {
		if len(reply.Choices) == 0 {
			err = fmt.Errorf("groq returned no choices")
		} else {
			call.RawOutput = reply.Choices[0].Message.Content
			err = json.Unmarshal([]byte(call.RawOutput), out)
		}
	}
	call.OK = err == nil
	if err != nil {
		call.Error = err.Error()
	}
	if logErr := events.LogAICall(ctx, g.db, call); logErr != nil {
		g.logger.Error("logging an AI call", "error", logErr)
	}
	return err
}

func (g *Groq) post(ctx context.Context, body []byte) (chatResponse, error) {
	var reply chatResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url, bytes.NewReader(body))
	if err != nil {
		return reply, err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := g.client.Do(req)
	if err != nil {
		return reply, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return reply, err
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return reply, fmt.Errorf("groq answered %d with a body that is not JSON", res.StatusCode)
	}
	if res.StatusCode != http.StatusOK {
		return reply, fmt.Errorf("groq answered %d: %s", res.StatusCode, reply.Error.Message)
	}
	return reply, nil
}
