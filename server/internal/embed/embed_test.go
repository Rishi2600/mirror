package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeServer(t *testing.T, dims int, modelID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/info" {
			json.NewEncoder(w).Encode(map[string]string{"model_id": modelID})
			return
		}
		var body struct {
			Inputs    []string `json:"inputs"`
			Normalize bool     `json:"normalize"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if !body.Normalize {
			t.Error("request did not ask for normalised vectors")
		}
		out := make([][]float32, len(body.Inputs))
		for i := range out {
			out[i] = make([]float32, dims)
			out[i][0] = 1
		}
		json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEmbed(t *testing.T) {
	tei := NewTEI(fakeServer(t, Dims, Model).URL)
	vectors, err := tei.Embed(context.Background(), []string{"query: a", "query: b"})
	if err != nil || len(vectors) != 2 || len(vectors[0]) != Dims {
		t.Fatalf("Embed = %d vectors, %v", len(vectors), err)
	}
	if err := tei.Check(context.Background()); err != nil {
		t.Errorf("Check: %v", err)
	}
}

func TestWrongDimensionsOrModelAreRejected(t *testing.T) {
	if _, err := NewTEI(fakeServer(t, 768, Model).URL).Embed(context.Background(), []string{"x"}); err == nil {
		t.Error("Embed accepted 768-dimension vectors")
	}
	err := NewTEI(fakeServer(t, Dims, "some/other-model").URL).Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "some/other-model") {
		t.Errorf("Check error = %v, want it to name the wrong model", err)
	}
}

func TestLiteral(t *testing.T) {
	if got := Literal([]float32{0.5, -0.25, 1}); got != "[0.5,-0.25,1]" {
		t.Errorf("Literal = %q", got)
	}
}
