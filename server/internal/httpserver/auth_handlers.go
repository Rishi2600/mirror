package httpserver

import (
	"errors"
	"net/http"
	"strconv"

	"mirror/server/internal/auth"
	"mirror/server/internal/users"
)

const sessionCookie = "mirror_session"

// UserResponse is the user as the API returns it.
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// Role is "" until the user picks candidate or employer.
	Role      string `json:"role"`
	FirstName string `json:"first_name"`
}

func toUserResponse(u users.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, Role: string(u.Role), FirstName: u.FirstName}
}

// POST /v1/auth/otp/request {email}
func (s *server) requestCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !readJSON(w, r, &body) {
		return
	}

	err := auth.RequestCode(r.Context(), s.db, s.mailer, body.Email)
	var tooSoon auth.TooSoonError
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, auth.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.As(err, &tooSoon):
		w.Header().Set("Retry-After", strconv.Itoa(int(tooSoon.RetryAfter.Seconds())))
		writeError(w, http.StatusTooManyRequests, err.Error())
	default:
		s.serverError(w, r, err)
	}
}

// POST /v1/auth/otp/verify {email, code}
func (s *server) verifyCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		TimeZone string `json:"time_zone"`
	}
	if !readJSON(w, r, &body) {
		return
	}

	user, err := auth.VerifyCode(r.Context(), s.db, body.Email, body.Code)
	if errors.Is(err, auth.ErrWrongCode) || errors.Is(err, auth.ErrCodeExpired) || errors.Is(err, auth.ErrTooManyTries) {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	if err := users.SetTimeZone(r.Context(), s.db, user.ID, body.TimeZone); err != nil {
		s.serverError(w, r, err)
		return
	}
	token, err := auth.CreateSession(r.Context(), s.db, user.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.SetCookie(w, s.sessionCookie(token, int(auth.SessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

// POST /v1/auth/logout
func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := auth.DeleteSession(r.Context(), s.db, c.Value); err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	http.SetCookie(w, s.sessionCookie("", -1)) // a negative MaxAge deletes the cookie
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/me
func (s *server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

// POST /v1/me/role {role}: candidate or employer, picked once.
func (s *server) setRole(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	var body struct {
		Role users.Role `json:"role"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Role != users.RoleCandidate && body.Role != users.RoleEmployer {
		writeError(w, http.StatusBadRequest, "role must be candidate or employer")
		return
	}

	err := users.SetRole(r.Context(), s.db, user.ID, body.Role)
	if errors.Is(err, users.ErrRoleSet) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	user.Role = body.Role
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

// currentUser returns the signed-in user, or writes a 401 and returns false.
func (s *server) currentUser(w http.ResponseWriter, r *http.Request) (users.User, bool) {
	token := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		token = c.Value
	}
	user, err := auth.UserForSession(r.Context(), s.db, token)
	if errors.Is(err, auth.ErrNoSession) || errors.Is(err, users.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "sign in first")
		return users.User{}, false
	}
	if err != nil {
		s.serverError(w, r, err)
		return users.User{}, false
	}
	return user, true
}

func (s *server) sessionCookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	}
}
