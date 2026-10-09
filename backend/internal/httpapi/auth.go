package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"workshop/api/internal/auth"
	"workshop/api/internal/store"
)

// tokenTTL is how long an issued bearer token stays valid.
const tokenTTL = 12 * time.Hour

// dummyPasswordHash is compared when the e-mail is unknown so a failed login
// costs the same wall-clock time whether or not the account exists. It is a
// throw-away bcrypt hash of a non-credential string, kept only to equalise the
// timing, and its comparison result is discarded.
const dummyPasswordHash = "$2a$10$H09Nj2BJdSqYHWzKmsu0ueksCEov0mnjFozPQskuST9hGUucoP3w2"

// loginRequest is the body of POST /api/auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse is the 200 body: the signed token and the public employee
// representation. It carries neither the password nor its hash (AC-30).
type loginResponse struct {
	Token    string           `json:"token"`
	Employee employeeResponse `json:"employee"`
}

// employeeResponse is the public employee shape of the API. It has no password
// field by construction, so no employee response can carry one (AC-30).
type employeeResponse struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// login handles POST /api/auth/login. It looks the employee up, verifies the
// stored bcrypt hash and issues a bearer token. Wrong credentials answer 401 in
// the uniform error body; the client address is limited to ten attempts per
// minute, beyond which it answers 429 (AC-28).
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.loginLimiter.Allow(clientKey(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many login attempts, try again later")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid email or password")
		return
	}

	employee, err := s.store.FindEmployeeByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, store.ErrEmployeeNotFound) {
			// Equalise the timing with a real comparison, then answer the same
			// 401 so the response does not reveal whether the e-mail exists.
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(employee.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid email or password")
		return
	}

	token, err := auth.Sign(s.cfg.AuthSecret, auth.Claims{
		Sub:   employee.ID,
		Email: employee.Email,
		Exp:   time.Now().Add(tokenTTL).Unix(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token: token,
		Employee: employeeResponse{
			Email: employee.Email,
			Name:  employee.Name,
		},
	})
}

// clientKey identifies the calling client for the login rate limit. It uses the
// remote host of the request; behind an untrusted proxy that is the proxy, the
// conservative fallback when no forwarded header is trusted.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
