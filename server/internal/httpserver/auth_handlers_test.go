package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"gorm.io/gorm"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/storage"
)

// fakeMailer records the last email instead of sending it.
type fakeMailer struct{ body string }

func (m *fakeMailer) Send(ctx context.Context, to, subject, body string) error {
	m.body = body
	return nil
}

// fakeQueue records queued jobs instead of running them.
type fakeQueue struct {
	fileIDs  []string
	embedded []string
}

func (q *fakeQueue) ParseCV(ctx context.Context, userID, fileID string) error {
	q.fileIDs = append(q.fileIDs, fileID)
	return nil
}

func (q *fakeQueue) EmbedCandidate(ctx context.Context, userID string) error {
	q.embedded = append(q.embedded, "candidate:"+userID)
	return nil
}

func (q *fakeQueue) EmbedHiringCard(ctx context.Context, cardID string) error {
	q.embedded = append(q.embedded, "hiring_card:"+cardID)
	return nil
}

// authClient is a test HTTP client with a cookie jar, like a browser.
type authClient struct {
	t      *testing.T
	base   string
	http   *http.Client
	mailer *fakeMailer
	queue  *fakeQueue
	tx     *gorm.DB
	files  *storage.Local
}

func newAuthClient(t *testing.T) *authClient {
	t.Helper()
	mailer := &fakeMailer{}
	queue := &fakeQueue{}
	tx := dbtest.Tx(t)
	files, err := storage.NewLocal(t.TempDir(), bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(Options{
		Logger:     slog.New(slog.DiscardHandler),
		DB:         tx,
		Mailer:     mailer,
		Files:      files,
		CVQueue:    queue,
		JobParser:  fakeJobParser{},
		EmbedQueue: queue,
		SigningKey: bytes.Repeat([]byte{2}, 32),
	}))
	t.Cleanup(srv.Close)
	return &authClient{t: t, base: srv.URL, http: &http.Client{Jar: newJar()}, mailer: mailer, queue: queue, tx: tx, files: files}
}

// newJar is an empty cookie jar: a new browser with nobody signed in.
func newJar() http.CookieJar {
	jar, _ := cookiejar.New(nil)
	return jar
}

// upload posts data as a multipart form file in field, like a browser form.
func (c *authClient) upload(path, field string, data []byte, out any) *http.Response {
	c.t.Helper()
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	part, _ := form.CreateFormFile(field, "cv.pdf")
	part.Write(data)
	form.Close()
	res, err := c.http.Post(c.base+path, form.FormDataContentType(), &buf)
	if err != nil {
		c.t.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res
}

// signInAs signs in a new user and picks role.
func (c *authClient) signInAs(email, role string) {
	c.t.Helper()
	c.signIn(email)
	if res := c.do("POST", "/v1/me/role", map[string]string{"role": role}, nil); res.StatusCode != http.StatusOK {
		c.t.Fatalf("picking %s: status %d", role, res.StatusCode)
	}
}

// do sends a request with an optional JSON body and decodes a JSON reply into out.
func (c *authClient) do(method, path string, body, out any) *http.Response {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, c.base+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res
}

// signIn requests a code and verifies it, returning the verify response.
func (c *authClient) signIn(email string) (*http.Response, UserResponse) {
	c.t.Helper()
	if res := c.do("POST", "/v1/auth/otp/request", map[string]string{"email": email}, nil); res.StatusCode != http.StatusNoContent {
		c.t.Fatalf("request code: status %d, want 204", res.StatusCode)
	}
	code := regexp.MustCompile(`\d{6}`).FindString(c.mailer.body)
	var user UserResponse
	res := c.do("POST", "/v1/auth/otp/verify", map[string]string{"email": email, "code": code}, &user)
	return res, user
}

func TestSignInFlow(t *testing.T) {
	c := newAuthClient(t)

	if res := c.do("GET", "/v1/me", nil, nil); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /v1/me before sign-in: status %d, want 401", res.StatusCode)
	}

	res, user := c.signIn("flow@example.com")
	if res.StatusCode != http.StatusOK || user.Email != "flow@example.com" || user.Role != "" {
		t.Fatalf("verify: status %d, user %+v", res.StatusCode, user)
	}
	cookie := res.Header.Get("Set-Cookie")
	for _, want := range []string{"mirror_session=", "HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(cookie, want) {
			t.Errorf("Set-Cookie %q is missing %q", cookie, want)
		}
	}

	var me UserResponse
	if res := c.do("GET", "/v1/me", nil, &me); res.StatusCode != http.StatusOK || me.ID != user.ID {
		t.Fatalf("GET /v1/me: status %d, user %+v", res.StatusCode, me)
	}

	if res := c.do("POST", "/v1/me/role", map[string]string{"role": "admin"}, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("picking admin: status %d, want 400", res.StatusCode)
	}
	if res := c.do("POST", "/v1/me/role", map[string]string{"role": "candidate"}, &me); res.StatusCode != http.StatusOK || me.Role != "candidate" {
		t.Errorf("picking candidate: status %d, user %+v", res.StatusCode, me)
	}
	if res := c.do("POST", "/v1/me/role", map[string]string{"role": "employer"}, nil); res.StatusCode != http.StatusConflict {
		t.Errorf("changing role: status %d, want 409", res.StatusCode)
	}

	if res := c.do("POST", "/v1/auth/logout", nil, nil); res.StatusCode != http.StatusNoContent {
		t.Errorf("logout: status %d, want 204", res.StatusCode)
	}
	if res := c.do("GET", "/v1/me", nil, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /v1/me after logout: status %d, want 401", res.StatusCode)
	}
}

func TestSignInErrors(t *testing.T) {
	c := newAuthClient(t)

	if res := c.do("POST", "/v1/auth/otp/request", map[string]string{"email": "nope"}, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("bad email: status %d, want 400", res.StatusCode)
	}

	c.do("POST", "/v1/auth/otp/request", map[string]string{"email": "errors@example.com"}, nil)
	res := c.do("POST", "/v1/auth/otp/request", map[string]string{"email": "errors@example.com"}, nil)
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") == "" {
		t.Errorf("second request: status %d, Retry-After %q; want 429 with Retry-After", res.StatusCode, res.Header.Get("Retry-After"))
	}

	var reply map[string]string
	res = c.do("POST", "/v1/auth/otp/verify", map[string]string{"email": "errors@example.com", "code": "not-it"}, &reply)
	if res.StatusCode != http.StatusUnauthorized || reply["error"] == "" {
		t.Errorf("wrong code: status %d, body %v; want 401 with an error", res.StatusCode, reply)
	}
}
