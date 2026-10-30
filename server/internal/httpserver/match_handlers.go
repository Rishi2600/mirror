package httpserver

import (
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"mirror/server/internal/chat"
	"mirror/server/internal/cvtext"
	"mirror/server/internal/hiring"
	"mirror/server/internal/live"
	"mirror/server/internal/profiles"
	"mirror/server/internal/selectivity"
	"mirror/server/internal/signing"
	"mirror/server/internal/swipes"
	"mirror/server/internal/users"
)

// cvLinkLife is how long a CV download link works (spec: 15 minutes).
const cvLinkLife = 15 * time.Minute

// SwipeResponse says whether the swipe made a match.
type SwipeResponse struct {
	Matched bool   `json:"matched"`
	MatchID string `json:"match_id"`
}

// MatchResponse is one match, as each side sees it.
type MatchResponse struct {
	ID        string    `json:"id"`
	MatchedAt time.Time `json:"matched_at"`
	// For candidates: the hiring card they matched.
	Job *hiring.View `json:"job,omitempty"`
	// For employers: which of their cards, and the candidate's card.
	HiringCardTitle string         `json:"hiring_card_title,omitempty"`
	Candidate       *profiles.Card `json:"candidate,omitempty"`
	// Contact is revealed to the employer only, on the match page.
	Contact *Contact `json:"contact,omitempty"`
	// The inbox fields, on the list only. New means not opened yet.
	New         bool             `json:"new"`
	Unread      int              `json:"unread"`
	LastMessage *MessageResponse `json:"last_message,omitempty"`
}

// Contact is what a match reveals to the employer.
type Contact struct {
	FullName string      `json:"full_name"`
	Email    string      `json:"email"`
	Phone    string      `json:"phone"`
	CV       profiles.CV `json:"cv"`
	// CVURL downloads the CV file; it expires after 15 minutes.
	CVURL string `json:"cv_url"`
}

// POST /v1/swipes {hiring_card_id, candidate_id, direction} with an
// Idempotency-Key header. A candidate's candidate_id is always their own.
func (s *server) createSwipe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	var body struct {
		HiringCardID string `json:"hiring_card_id"`
		CandidateID  string `json:"candidate_id"`
		Direction    string `json:"direction"`
		DwellMS      int    `json:"dwell_ms"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if user.Role == users.RoleCandidate {
		body.CandidateID = user.ID
	}

	res, err := swipes.Record(r.Context(), s.db, user, body.HiringCardID, body.CandidateID, body.Direction, r.Header.Get("Idempotency-Key"), body.DwellMS)
	var limited *selectivity.Limited
	switch {
	case errors.As(err, &limited):
		// The spec: a refused like answers 429 with the time it ends.
		retry := int(math.Ceil(time.Until(limited.Until).Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": limited.Reason, "until": limited.Until})
	case errors.Is(err, swipes.ErrDirection), errors.Is(err, swipes.ErrKey):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, swipes.ErrKeyReused):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, swipes.ErrAlreadySwiped):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, swipes.ErrNotAllowed):
		writeError(w, http.StatusForbidden, err.Error())
	case err != nil:
		s.serverError(w, r, err)
	default:
		out := SwipeResponse{Matched: res.Match != nil}
		if res.Match != nil {
			out.MatchID = res.Match.ID
			s.notifyMatch(r, user, *res.Match)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// SelectivityResponse is what the browser needs for nudges. Candidates get
// their likes; employers learn if they are ranking lower for not replying.
// The raw score is never shown.
type SelectivityResponse struct {
	Likes    *selectivity.Likes `json:"likes,omitempty"`
	Ghosting bool               `json:"ghosting"`
}

// GET /v1/selectivity
func (s *server) getSelectivity(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	var out SelectivityResponse
	if user.Role == users.RoleEmployer {
		ghosting, err := selectivity.Ghosting(r.Context(), s.db, []string{user.ID}, s.now())
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		out.Ghosting = ghosting[user.ID]
	} else {
		likes, err := selectivity.LikesFor(r.Context(), s.db, user, s.now())
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		out.Likes = &likes
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/matches: the user's live matches, the latest activity first.
func (s *server) listMatches(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	list, err := swipes.Matches(r.Context(), s.db, user)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	ids := make([]string, len(list))
	for i, m := range list {
		ids[i] = m.ID
	}
	summaries, err := chat.Summaries(r.Context(), s.db, user.ID, ids)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	out := []MatchResponse{}
	lastActive := map[string]time.Time{}
	for _, m := range list {
		resp, err := s.matchResponse(r, user, m, false)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		mine, _ := m.ReadAt(user.ID)
		sum := summaries[m.ID]
		resp.New, resp.Unread = mine == nil, sum.Unread
		lastActive[m.ID] = m.MatchedAt
		if sum.Last != nil {
			last := toMessageResponse(*sum.Last)
			resp.LastMessage = &last
			lastActive[m.ID] = sum.Last.CreatedAt
		}
		out = append(out, resp)
	}
	sort.SliceStable(out, func(i, j int) bool { return lastActive[out[i].ID].After(lastActive[out[j].ID]) })
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/matches/{id}: one match. The employer also gets the contact
// details, the full profile and a CV link.
func (s *server) getMatch(w http.ResponseWriter, r *http.Request) {
	user, m, ok := s.ownMatch(w, r)
	if !ok {
		return
	}
	resp, err := s.matchResponse(r, user, m, true)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// DELETE /v1/matches/{id}: either side ends the match. The employer loses
// the contact details and CV, and the chat closes for both.
func (s *server) unmatch(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	m, err := swipes.Unmatch(r.Context(), s.db, user, r.PathValue("id"))
	if errors.Is(err, swipes.ErrMatchNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	e := live.Event{Type: live.TypeUnmatched, MatchID: m.ID}
	s.hub.Send(m.CandidateUserID, e)
	s.hub.Send(m.EmployerUserID, e)
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/matches/{id}/cv?exp=...&sig=...: the CV file, for the match's
// employer, while the signed link is valid.
func (s *server) downloadCV(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireEmployer(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	err := signing.Verify(s.signingKey, r.URL.Path, q.Get("exp"), q.Get("sig"), time.Now())
	if errors.Is(err, signing.ErrExpired) {
		writeError(w, http.StatusGone, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	m, err := swipes.Get(r.Context(), s.db, user, r.PathValue("id"))
	if err != nil || m.EmployerUserID != user.ID {
		writeError(w, http.StatusNotFound, swipes.ErrMatchNotFound.Error())
		return
	}
	p, err := profiles.Get(r.Context(), s.db, m.CandidateUserID)
	if err != nil || p.CVFileID == nil {
		writeError(w, http.StatusNotFound, "this candidate has no CV file")
		return
	}
	var file profiles.File
	if err := s.db.WithContext(r.Context()).Where("id = ?", *p.CVFileID).First(&file).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	data, err := s.files.Open(file.StorageKey)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := swipes.Audit(r.Context(), s.db, user.ID, "cv_download", m.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	name := "cv.pdf"
	if file.ContentType == cvtext.TypeDOCX {
		name = "cv.docx"
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(data)
}

// ownMatch loads the match in the path if the user is part of it.
func (s *server) ownMatch(w http.ResponseWriter, r *http.Request) (users.User, swipes.Match, bool) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return users.User{}, swipes.Match{}, false
	}
	m, err := swipes.Get(r.Context(), s.db, user, r.PathValue("id"))
	if errors.Is(err, swipes.ErrMatchNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return users.User{}, swipes.Match{}, false
	}
	if err != nil {
		s.serverError(w, r, err)
		return users.User{}, swipes.Match{}, false
	}
	return user, m, true
}

// matchResponse builds what this user sees of a match; withContact adds
// the employer's reveal (the match page, not the list).
func (s *server) matchResponse(r *http.Request, user users.User, m swipes.Match, withContact bool) (MatchResponse, error) {
	ctx := r.Context()
	out := MatchResponse{ID: m.ID, MatchedAt: m.MatchedAt}
	card, err := hiring.ByID(ctx, s.db, m.HiringCardID)
	if err != nil {
		return out, err
	}

	if user.Role != users.RoleEmployer {
		employer, err := users.ByID(ctx, s.db, m.EmployerUserID)
		if err != nil {
			return out, err
		}
		view, _ := hiring.BuildView(card, employer.FirstName, jobImageURL(card))
		out.Job = &view
		return out, nil
	}

	p, err := profiles.Get(ctx, s.db, m.CandidateUserID)
	if err != nil {
		return out, err
	}
	candidate, _ := profiles.BuildCard(p, candidatePhotoURL(p))
	out.HiringCardTitle = card.Title
	out.Candidate = &candidate
	if !withContact {
		return out, nil
	}

	account, err := users.ByID(ctx, s.db, m.CandidateUserID)
	if err != nil {
		return out, err
	}
	out.Contact = &Contact{FullName: p.CV.FullName, Email: account.Email, Phone: p.CV.Phone, CV: p.CV}
	if p.CVFileID != nil {
		out.Contact.CVURL = signing.Sign(s.signingKey, "/v1/matches/"+m.ID+"/cv", time.Now().Add(cvLinkLife))
	}
	return out, nil
}
