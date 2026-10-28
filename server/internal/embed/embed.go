// Package embed turns text into meaning vectors using the local embedding
// server (Hugging Face Text Embeddings Inference, see compose.yaml).
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Model and Dims must match compose.yaml and the embeddings table.
const (
	Model = "intfloat/multilingual-e5-small"
	Dims  = 384
)

// Embedder turns texts into unit-length vectors, one per text.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Prefix marks a text for E5 models. Candidate cards and hiring cards are
// matched both ways, so both get the "query: " prefix the model expects for
// symmetric matching.
func Prefix(text string) string {
	return "query: " + text
}

// TEI calls a Text Embeddings Inference server.
type TEI struct {
	url    string
	client *http.Client
}

func NewTEI(url string) *TEI {
	return &TEI{url: strings.TrimRight(url, "/"), client: &http.Client{Timeout: 30 * time.Second}}
}

// Embed returns one vector per text, normalised to length 1.
func (t *TEI) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"inputs": texts, "normalize": true, "truncate": true})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding server: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding server answered %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	var vectors [][]float32
	if err := json.Unmarshal(data, &vectors); err != nil {
		return nil, fmt.Errorf("embedding server reply: %w", err)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("embedding server returned %d vectors for %d texts", len(vectors), len(texts))
	}
	for _, v := range vectors {
		if len(v) != Dims {
			return nil, fmt.Errorf("embedding server returned %d dimensions, want %d", len(v), Dims)
		}
	}
	return vectors, nil
}

// Check confirms the server runs the model the database expects.
func (t *TEI) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url+"/info", nil)
	if err != nil {
		return err
	}
	res, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("embedding server: %w", err)
	}
	defer res.Body.Close()
	var info struct {
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return fmt.Errorf("embedding server info: %w", err)
	}
	if info.ModelID != Model {
		return fmt.Errorf("embedding server runs %q, want %q", info.ModelID, Model)
	}
	return nil
}

// Literal writes a vector the way pgvector reads it: "[0.1,0.2,...]".
func Literal(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
