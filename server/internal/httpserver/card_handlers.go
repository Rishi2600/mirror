package httpserver

import (
	"errors"
	"io"
	"net/http"

	"mirror/server/internal/photos"
	"mirror/server/internal/profiles"
	"mirror/server/internal/scrub"
)

const maxPhotoBytes = 5 << 20 // 5 MB

// CardResponse is the candidate's own card preview.
type CardResponse struct {
	// Card is exactly what employers see.
	Card profiles.Card `json:"card"`
	// Removed counts contact details the scrub took out of the card's text.
	Removed scrub.Removed `json:"removed"`
	// Missing lists what the card needs before it can be finished.
	Missing   []string `json:"missing"`
	Onboarded bool     `json:"onboarded"`
}

// POST /v1/profile/photo: multipart form with the image in field "photo".
func (s *server) uploadPhoto(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireCandidate(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes+(1<<20))
	file, _, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "send the photo as a form file named photo (JPEG, PNG or WebP, up to 5 MB)")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoBytes+1))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if len(data) > maxPhotoBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "the photo must be 5 MB or smaller")
		return
	}

	p, err := profiles.SetPhoto(r.Context(), s.db, s.files, user.ID, data)
	switch {
	case errors.Is(err, profiles.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, photos.ErrNotImage):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		s.serverError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, toProfileResponse(p))
	}
}

// GET /v1/profile/photo: the candidate's own photo.
func (s *server) getPhoto(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireCandidate(w, r)
	if !ok {
		return
	}
	jpg, err := profiles.Photo(r.Context(), s.db, s.files, user.ID)
	if errors.Is(err, profiles.ErrNotFound) || errors.Is(err, profiles.ErrNoPhoto) {
		writeError(w, http.StatusNotFound, "no photo yet")
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	// Only this user's browser may keep a copy.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(jpg)
}

// GET /v1/card: the candidate's card, exactly as employers will see it.
func (s *server) getCard(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireCandidate(w, r)
	if !ok {
		return
	}
	p, err := profiles.Get(r.Context(), s.db, user.ID)
	if errors.Is(err, profiles.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	card, removed := profiles.BuildCard(p, ownPhotoURL(p))
	missing := profiles.Missing(p)
	if missing == nil {
		missing = []string{}
	}
	writeJSON(w, http.StatusOK, CardResponse{Card: card, Removed: removed, Missing: missing, Onboarded: p.OnboardedAt != nil})
}

// POST /v1/card/finish: the last onboarding step; the card goes live.
func (s *server) finishCard(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireCandidate(w, r)
	if !ok {
		return
	}
	p, err := profiles.Finish(r.Context(), s.db, user.ID)
	var missing profiles.MissingError
	switch {
	case errors.Is(err, profiles.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.As(err, &missing):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		s.serverError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, toProfileResponse(p))
	}
}
