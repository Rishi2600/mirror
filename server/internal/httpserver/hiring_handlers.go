package httpserver

import (
	"errors"
	"io"
	"net/http"
	"time"

	"mirror/server/internal/hiring"
	"mirror/server/internal/photos"
	"mirror/server/internal/scrub"
	"mirror/server/internal/users"
)

// HiringCardResponse is one of the employer's own hiring cards.
type HiringCardResponse struct {
	ID string `json:"id"`
	// Status is draft, live or closed.
	Status string `json:"status"`
	hiring.Fields
	Description string `json:"description"`
	// ImageURL is "" until an image is uploaded.
	ImageURL  string    `json:"image_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HiringPreviewResponse is a card exactly as candidates will see it.
type HiringPreviewResponse struct {
	View    hiring.View   `json:"view"`
	Removed scrub.Removed `json:"removed"`
	// Missing lists what the card needs before it can go live.
	Missing []string `json:"missing"`
}

func toHiringCardResponse(c hiring.Card) HiringCardResponse {
	return HiringCardResponse{
		ID:          c.ID,
		Status:      c.Status,
		Fields:      c.Fields(),
		Description: c.Description,
		ImageURL:    cardImageURL(c),
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// cardImageURL carries the image's file ID, so a new image shows at once.
func cardImageURL(c hiring.Card) string {
	if c.ImageFileID == nil {
		return ""
	}
	return "/v1/hiring-cards/" + c.ID + "/image?v=" + *c.ImageFileID
}

// GET /v1/hiring-cards: the employer's cards, newest first.
func (s *server) listHiringCards(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	cards, err := hiring.List(r.Context(), s.db, user.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	out := []HiringCardResponse{}
	for _, c := range cards {
		out = append(out, toHiringCardResponse(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /v1/hiring-cards {description}: the AI reads the job description
// into a new draft card.
func (s *server) createHiringCard(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	var body struct {
		Description string `json:"description"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	c, err := hiring.Create(r.Context(), s.db, s.jobParser, user.ID, body.Description)
	if errors.Is(err, hiring.ErrDescription) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.logger.Error("creating a hiring card", "error", err)
		writeError(w, http.StatusBadGateway, "we could not read this job description right now; please try again")
		return
	}
	writeJSON(w, http.StatusCreated, toHiringCardResponse(c))
}

// GET /v1/hiring-cards/{id}
func (s *server) getHiringCard(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	c, ok := s.ownCard(w, r, user)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toHiringCardResponse(c))
}

// PUT /v1/hiring-cards/{id}: replaces the card's fields with the edits.
func (s *server) updateHiringCard(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	var f hiring.Fields
	if !readJSON(w, r, &f) {
		return
	}
	if err := hiring.Validate(f); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := hiring.Update(r.Context(), s.db, user.ID, r.PathValue("id"), f)
	if err == nil {
		s.queueCardVector(r, c)
	}
	s.writeCard(w, r, c, err)
}

// PUT /v1/hiring-cards/{id}/status {status}: live (shown to candidates) or
// closed (hidden).
func (s *server) setHiringCardStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	c, err := hiring.SetStatus(r.Context(), s.db, user.ID, r.PathValue("id"), body.Status, user.FirstName)
	var missing hiring.MissingError
	if errors.Is(err, hiring.ErrStatus) || errors.As(err, &missing) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err == nil {
		s.queueCardVector(r, c)
	}
	s.writeCard(w, r, c, err)
}

// POST /v1/hiring-cards/{id}/image: multipart form with field "image".
func (s *server) uploadHiringCardImage(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes+(1<<20))
	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "send the image as a form file named image (JPEG, PNG or WebP, up to 5 MB)")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoBytes+1))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if len(data) > maxPhotoBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "the image must be 5 MB or smaller")
		return
	}
	c, err := hiring.SetImage(r.Context(), s.db, s.files, user.ID, r.PathValue("id"), data)
	if errors.Is(err, photos.ErrNotImage) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeCard(w, r, c, err)
}

// GET /v1/hiring-cards/{id}/image: for the card's employer for now; the
// candidates' timeline adds its own access in chunk 7.
func (s *server) getHiringCardImage(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	c, ok := s.ownCard(w, r, user)
	if !ok {
		return
	}
	jpg, err := hiring.Image(r.Context(), s.db, s.files, c)
	if errors.Is(err, hiring.ErrNoImage) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(jpg)
}

// GET /v1/hiring-cards/{id}/preview: the card as candidates will see it.
func (s *server) previewHiringCard(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	c, ok := s.ownCard(w, r, user)
	if !ok {
		return
	}
	view, removed := hiring.BuildView(c, user.FirstName, cardImageURL(c))
	missing := hiring.Missing(c, user.FirstName)
	if missing == nil {
		missing = []string{}
	}
	writeJSON(w, http.StatusOK, HiringPreviewResponse{View: view, Removed: removed, Missing: missing})
}

// PUT /v1/me {first_name}
func (s *server) updateMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	var body struct {
		FirstName string `json:"first_name"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if err := users.SetFirstName(r.Context(), s.db, user.ID, body.FirstName); err != nil {
		s.serverError(w, r, err)
		return
	}
	user, err := users.ByID(r.Context(), s.db, user.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

// requireEmployer is currentUser for employer-only routes (403 otherwise).
func (s *server) requireEmployer(w http.ResponseWriter, r *http.Request) (users.User, bool) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return users.User{}, false
	}
	if user.Role != users.RoleEmployer {
		writeError(w, http.StatusForbidden, "only employers have hiring cards")
		return users.User{}, false
	}
	return user, true
}

// ownCard loads the card in the path if it belongs to the employer; any
// other card is a 404.
func (s *server) ownCard(w http.ResponseWriter, r *http.Request, user users.User) (hiring.Card, bool) {
	c, err := hiring.Get(r.Context(), s.db, user.ID, r.PathValue("id"))
	if errors.Is(err, hiring.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return hiring.Card{}, false
	}
	if err != nil {
		s.serverError(w, r, err)
		return hiring.Card{}, false
	}
	return c, true
}

// writeCard answers with the card, or with the error from loading it.
func (s *server) writeCard(w http.ResponseWriter, r *http.Request, c hiring.Card, err error) {
	switch {
	case errors.Is(err, hiring.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		s.serverError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, toHiringCardResponse(c))
	}
}
