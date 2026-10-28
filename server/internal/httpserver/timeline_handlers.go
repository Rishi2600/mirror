package httpserver

import (
	"context"
	"errors"
	"net/http"

	"mirror/server/internal/hiring"
	"mirror/server/internal/matching"
	"mirror/server/internal/profiles"
)

// EmbedQueue queues vector refreshes: jobs.Queue in the server, a fake in
// tests.
type EmbedQueue interface {
	EmbedCandidate(ctx context.Context, userID string) error
	EmbedHiringCard(ctx context.Context, cardID string) error
}

// TimelineJob is one hiring card on a candidate's timeline.
type TimelineJob struct {
	CardID string      `json:"card_id"`
	View   hiring.View `json:"view"`
	// Score is the pair score in [0, 1]; Reasons say why the card is here.
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons"`
}

// TimelineCandidate is one candidate on a hiring card's timeline.
type TimelineCandidate struct {
	CandidateID string        `json:"candidate_id"`
	Card        profiles.Card `json:"card"`
	Score       float64       `json:"score"`
	Reasons     []string      `json:"reasons"`
}

// GET /v1/timeline: the live hiring cards that fit the candidate, best first.
func (s *server) candidateTimeline(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireCandidate(w, r)
	if !ok {
		return
	}
	p, err := profiles.Get(r.Context(), s.db, user.ID)
	if errors.Is(err, profiles.ErrNotFound) || err == nil && p.OnboardedAt == nil {
		writeError(w, http.StatusConflict, "finish your card first; then fitting jobs show up here")
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	matches, err := matching.CandidateTimeline(r.Context(), s.db, p)
	if errors.Is(err, matching.ErrNoVector) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	out := []TimelineJob{}
	for _, m := range matches {
		view, _ := hiring.BuildView(m.Card, m.FirstName, jobImageURL(m.Card))
		out = append(out, TimelineJob{CardID: m.Card.ID, View: view, Score: m.Score, Reasons: matching.Reasons(m.Scores)})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/hiring-cards/{id}/timeline: the candidates that fit one of the
// employer's live cards, best first.
func (s *server) hiringCardTimeline(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	c, ok := s.ownCard(w, r, user)
	if !ok {
		return
	}
	if c.Status != hiring.StatusLive {
		writeError(w, http.StatusConflict, "put this card live first; then fitting candidates show up here")
		return
	}
	matches, err := matching.HiringCardTimeline(r.Context(), s.db, c)
	if errors.Is(err, matching.ErrNoVector) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	out := []TimelineCandidate{}
	for _, m := range matches {
		card, _ := profiles.BuildCard(m.Profile, candidatePhotoURL(m.Profile))
		out = append(out, TimelineCandidate{CandidateID: m.Profile.UserID, Card: card, Score: m.Score, Reasons: matching.Reasons(m.Scores)})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/jobs/{id}/image: a live hiring card's image, for candidates.
func (s *server) jobImage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireCandidate(w, r); !ok {
		return
	}
	var c hiring.Card
	err := s.db.WithContext(r.Context()).Where("id = ? AND status = ?", r.PathValue("id"), hiring.StatusLive).First(&c).Error
	if err != nil {
		writeError(w, http.StatusNotFound, "no such job")
		return
	}
	jpg, err := hiring.Image(r.Context(), s.db, s.files, c)
	s.writeImage(w, r, jpg, err)
}

// GET /v1/candidates/{id}/photo: a finished candidate card's photo, for
// employers. The photo is part of the card employers see before a match.
func (s *server) candidatePhoto(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireEmployer(w, r); !ok {
		return
	}
	p, err := profiles.Get(r.Context(), s.db, r.PathValue("id"))
	if err != nil || p.OnboardedAt == nil {
		writeError(w, http.StatusNotFound, "no such candidate")
		return
	}
	jpg, err := profiles.Photo(r.Context(), s.db, s.files, p.UserID)
	s.writeImage(w, r, jpg, err)
}

func (s *server) writeImage(w http.ResponseWriter, r *http.Request, jpg []byte, err error) {
	if errors.Is(err, hiring.ErrNoImage) || errors.Is(err, profiles.ErrNoPhoto) {
		writeError(w, http.StatusNotFound, "no image")
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

func jobImageURL(c hiring.Card) string {
	if c.ImageFileID == nil {
		return ""
	}
	return "/v1/jobs/" + c.ID + "/image?v=" + *c.ImageFileID
}

func candidatePhotoURL(p profiles.Profile) string {
	if p.PhotoFileID == nil {
		return ""
	}
	return "/v1/candidates/" + p.UserID + "/photo?v=" + *p.PhotoFileID
}

// queueCandidateVector refreshes a finished card's vector after an edit. A
// failure is only logged: the next server start backfills missing vectors.
func (s *server) queueCandidateVector(r *http.Request, p profiles.Profile) {
	if p.OnboardedAt == nil || s.embedQueue == nil {
		return
	}
	if err := s.embedQueue.EmbedCandidate(r.Context(), p.UserID); err != nil {
		s.logger.Error("queueing a candidate vector", "user", p.UserID, "error", err)
	}
}

// queueCardVector refreshes a live hiring card's vector after an edit.
func (s *server) queueCardVector(r *http.Request, c hiring.Card) {
	if c.Status != hiring.StatusLive || s.embedQueue == nil {
		return
	}
	if err := s.embedQueue.EmbedHiringCard(r.Context(), c.ID); err != nil {
		s.logger.Error("queueing a hiring card vector", "card", c.ID, "error", err)
	}
}
