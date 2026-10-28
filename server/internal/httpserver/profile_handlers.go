package httpserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"mirror/server/internal/cvtext"
	"mirror/server/internal/profiles"
	"mirror/server/internal/users"
)

const maxCVBytes = 5 << 20 // 5 MB

// CVQueue queues CV parsing: jobs.Queue in the server, a fake in tests.
type CVQueue interface {
	ParseCV(ctx context.Context, userID, fileID string) error
}

// ProfileResponse is the candidate profile as the API returns it.
type ProfileResponse struct {
	// ParseStatus is pending (reading the CV), done or failed.
	ParseStatus  string                `json:"parse_status"`
	ParseError   string                `json:"parse_error"`
	CV           profiles.CV           `json:"cv"`
	Preferences  profiles.Preferences  `json:"preferences"`
	CardSettings profiles.CardSettings `json:"card_settings"`
	// PhotoURL is "" until a photo is uploaded.
	PhotoURL string `json:"photo_url"`
	// Onboarded is true once the card is finished (and live).
	Onboarded bool      `json:"onboarded"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toProfileResponse(p profiles.Profile) ProfileResponse {
	return ProfileResponse{
		ParseStatus:  p.ParseStatus,
		ParseError:   p.ParseError,
		CV:           p.CV,
		Preferences:  p.Preferences(),
		CardSettings: p.CardSettings(),
		PhotoURL:     ownPhotoURL(p),
		Onboarded:    p.OnboardedAt != nil,
		UpdatedAt:    p.UpdatedAt,
	}
}

// ownPhotoURL is where candidates load their own photo. The file ID in the
// query makes the browser fetch a new photo after an upload.
func ownPhotoURL(p profiles.Profile) string {
	if p.PhotoFileID == nil {
		return ""
	}
	return "/v1/profile/photo?v=" + *p.PhotoFileID
}

// POST /v1/profile/cv: multipart form with the file in field "cv".
func (s *server) uploadCV(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireCandidate(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCVBytes+(1<<20)) // room for the form fields
	file, _, err := r.FormFile("cv")
	if err != nil {
		writeError(w, http.StatusBadRequest, "send the CV as a form file named cv (PDF or DOCX, up to 5 MB)")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCVBytes+1))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if len(data) > maxCVBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "the CV must be 5 MB or smaller")
		return
	}

	p, fileID, err := profiles.UploadCV(r.Context(), s.db, s.files, user.ID, data)
	if errors.Is(err, cvtext.ErrUnsupported) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.cvQueue.ParseCV(r.Context(), user.ID, fileID); err != nil {
		profiles.MarkFailed(r.Context(), s.db, user.ID, fileID, "we could not start reading your CV; please upload it again")
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, toProfileResponse(p))
}

// GET /v1/profile
func (s *server) getProfile(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, toProfileResponse(p))
}

// PUT /v1/profile {cv, preferences, card_settings}: replaces all three with
// the candidate's edits.
func (s *server) updateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireCandidate(w, r)
	if !ok {
		return
	}
	var body struct {
		CV           profiles.CV           `json:"cv"`
		Preferences  profiles.Preferences  `json:"preferences"`
		CardSettings profiles.CardSettings `json:"card_settings"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if err := profiles.ValidatePreferences(body.Preferences); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := profiles.ValidateCardSettings(body.CardSettings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	p, err := profiles.Update(r.Context(), s.db, user.ID, body.CV, body.Preferences, body.CardSettings)
	switch {
	case errors.Is(err, profiles.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, profiles.ErrParsing):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		s.serverError(w, r, err)
	default:
		s.queueCandidateVector(r, p)
		writeJSON(w, http.StatusOK, toProfileResponse(p))
	}
}

// requireCandidate is currentUser for candidate-only routes (403 otherwise).
func (s *server) requireCandidate(w http.ResponseWriter, r *http.Request) (users.User, bool) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return users.User{}, false
	}
	if user.Role != users.RoleCandidate {
		writeError(w, http.StatusForbidden, "only candidates have a profile")
		return users.User{}, false
	}
	return user, true
}
