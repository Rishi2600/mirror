package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/events"
	"mirror/server/internal/hiring"
	"mirror/server/internal/profiles"
)

// fakeGroq answers like Groq's chat API and records the last request.
func fakeGroq(t *testing.T, status int, reply string) (*httptest.Server, *map[string]any) {
	t.Helper()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestParseCV(t *testing.T) {
	tx := dbtest.Tx(t)
	content, _ := json.Marshal(profiles.CV{FullName: "Jordan Example", Seniority: "senior", Skills: []string{"Go"}})
	reply, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}},
		"usage":   map[string]any{"prompt_tokens": 900, "completion_tokens": 300},
	})
	srv, got := fakeGroq(t, http.StatusOK, string(reply))

	g := NewGroq("test-key", "openai/gpt-oss-120b", tx, slog.New(slog.DiscardHandler))
	g.url = srv.URL
	cv, err := g.ParseCV(context.Background(), "Jordan Example\nSenior backend engineer")
	if err != nil {
		t.Fatalf("ParseCV: %v", err)
	}
	if cv.FullName != "Jordan Example" || cv.Seniority != "senior" {
		t.Errorf("cv = %+v", cv)
	}

	format := (*got)["response_format"].(map[string]any)["json_schema"].(map[string]any)
	if (*got)["model"] != "openai/gpt-oss-120b" || format["strict"] != true {
		t.Errorf("request did not ask for strict JSON from the model: %v", *got)
	}

	var call events.AICall
	tx.Where("purpose = ?", "parse_cv").Last(&call)
	if !call.OK || call.InputTokens != 900 || call.OutputTokens != 300 || !strings.Contains(call.RawOutput, "Jordan") {
		t.Errorf("logged call = %+v", call)
	}
}

func TestGroqErrorIsReturnedAndLogged(t *testing.T) {
	tx := dbtest.Tx(t)
	srv, _ := fakeGroq(t, http.StatusTooManyRequests, `{"error": {"message": "rate limit reached"}}`)

	g := NewGroq("test-key", "openai/gpt-oss-120b", tx, slog.New(slog.DiscardHandler))
	g.url = srv.URL
	_, err := g.ParseCV(context.Background(), "some cv")
	if err == nil || !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("error = %v, want the status and Groq's message", err)
	}

	var call events.AICall
	tx.Where("purpose = ?", "parse_cv").Last(&call)
	if call.OK || !strings.Contains(call.Error, "rate limit") {
		t.Errorf("logged call = %+v, want a failed call with the error", call)
	}
}

// Each schema's field names must match the JSON tags of its Go struct.
func TestSchemasMatchStructs(t *testing.T) {
	for name, tt := range map[string]struct {
		schema string
		value  any
	}{
		"cv":  {cvSchema, profiles.CV{}},
		"job": {jobSchema, hiring.Fields{}},
	} {
		var schema struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal([]byte(tt.schema), &schema); err != nil {
			t.Fatalf("%s schema is not valid JSON: %v", name, err)
		}

		var tags []string
		typ := reflect.TypeOf(tt.value)
		for i := range typ.NumField() {
			tags = append(tags, typ.Field(i).Tag.Get("json"))
		}
		slices.Sort(tags)
		required := slices.Sorted(slices.Values(schema.Required))
		if !slices.Equal(tags, required) || len(schema.Properties) != len(tags) {
			t.Errorf("%s schema fields %v do not match the struct fields %v", name, required, tags)
		}
	}
}
