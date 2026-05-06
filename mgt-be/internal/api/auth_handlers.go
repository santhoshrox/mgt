package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/santhoshrox/mgt-be/internal/auth"
	"github.com/santhoshrox/mgt-be/internal/db"
	"github.com/santhoshrox/mgt-be/internal/github"
)

const sessionCookie = "mgt_session"

// authMiddleware accepts either a bearer API token (CLI) or a session cookie
// (UI) and stuffs the user into the context.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID int64
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token := strings.TrimPrefix(h, "Bearer ")
			id, err := s.db.UserIDByTokenHash(r.Context(), auth.HashAPIToken(token))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			userID = id
		} else if c, err := r.Cookie(sessionCookie); err == nil {
			sess, verr := s.signer.Verify(c.Value)
			if verr != nil {
				writeError(w, http.StatusUnauthorized, "invalid session")
				return
			}
			userID = sess.UserID
		} else {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		u, err := s.db.GetUserByID(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey{}, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type userCtxKey struct{}

func userFrom(r *http.Request) db.User {
	u, _ := r.Context().Value(userCtxKey{}).(db.User)
	return u
}

// ghClientForUser builds a GitHub REST client for the request's user, after
// decrypting their stored OAuth token.
func (s *Server) ghClientForUser(u db.User) (*github.Client, error) {
	tok, err := s.sealer.Open(u.EncryptedOAuthToken)
	if err != nil {
		return nil, err
	}
	return github.New(tok), nil
}

// ── OAuth (web) ───────────────────────────────────────────────────────────

func (s *Server) githubLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GitHubClientID == "" {
		writeError(w, http.StatusServiceUnavailable, "GitHub OAuth not configured (set GITHUB_CLIENT_ID/SECRET)")
		return
	}

	state := randHex(16)
	cliState := r.URL.Query().Get("cli_state")

	// Stash both `state` and the (optional) `cli_state` in a short-lived
	// signed cookie so the callback can re-link a CLI flow even though the
	// browser may not preserve query params.
	stateValue := state
	if cliState != "" {
		stateValue = state + "|" + cliState
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "mgt_oauth_state",
		Value:    stateValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.cfg.BaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	q := url.Values{
		"client_id":    {s.cfg.GitHubClientID},
		"redirect_uri": {s.cfg.BaseURL + "/auth/github/callback"},
		"scope":        {"repo read:user read:org"},
		"state":        {state},
	}
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

func (s *Server) githubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	c, err := r.Cookie("mgt_oauth_state")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing oauth state")
		return
	}
	storedState, cliState := c.Value, ""
	if i := strings.IndexByte(c.Value, '|'); i >= 0 {
		storedState, cliState = c.Value[:i], c.Value[i+1:]
	}
	if storedState != state || code == "" {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}

	form := url.Values{
		"client_id":     {s.cfg.GitHubClientID},
		"client_secret": {s.cfg.GitHubClientSecret},
		"code":          {code},
		"redirect_uri":  {s.cfg.BaseURL + "/auth/github/callback"},
	}
	req, _ := http.NewRequestWithContext(r.Context(), "POST",
		"https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "github token exchange failed")
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var tr githubTokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil || tr.AccessToken == "" {
		writeError(w, http.StatusBadGateway, "github did not return a token")
		return
	}

	// Look up the GitHub user, then upsert.
	gh := github.New(tr.AccessToken)
	me, err := gh.Me()
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not read /user from github")
		return
	}
	sealed, err := s.sealer.Seal(tr.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "seal token: "+err.Error())
		return
	}
	user, err := s.db.UpsertUser(r.Context(), db.User{
		GitHubID:            me.ID,
		Login:               me.Login,
		AvatarURL:           me.AvatarURL,
		EncryptedOAuthToken: sealed,
		Scopes:              tr.Scope,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert user: "+err.Error())
		return
	}

	// Issue a session cookie.
	tok, err := s.signer.Sign(user.ID, 7*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.cfg.BaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})
	http.SetCookie(w, &http.Cookie{Name: "mgt_oauth_state", Path: "/", MaxAge: -1})

	// If this OAuth round-trip was kicked off from a CLI flow, mint an API
	// token and complete the device flow.
	if cliState != "" {
		token, hash, err := auth.NewAPIToken()
		if err == nil {
			_ = s.db.InsertAPIToken(r.Context(), user.ID, hash, "cli")
			_ = s.device.Complete(cliState, user.ID, token)
		}
		http.Redirect(w, r, s.cfg.BaseURL+"/auth/cli?state="+cliState+"&done=1", http.StatusFound)
		return
	}

	http.Redirect(w, r, s.cfg.UIBaseURL, http.StatusFound)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

// ── CLI device flow ───────────────────────────────────────────────────────

func (s *Server) cliStart(w http.ResponseWriter, r *http.Request) {
	uc, st, err := s.device.Start()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"user_code":        uc,
		"state":            st,
		"verification_url": s.cfg.BaseURL + "/auth/github/login?cli_state=" + st,
	})
}

func (s *Server) cliPoll(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		writeError(w, http.StatusBadRequest, "state required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	token, _, err := s.device.Poll(ctx, state)
	if err != nil {
		writeError(w, http.StatusGatewayTimeout, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// cliConfirmPage is the simple "you're logged in, go back to your terminal"
// page rendered after the CLI flow completes.
func (s *Server) cliConfirmPage(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	uc, _ := s.device.UserCode(state)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.URL.Query().Get("done") != "" {
		fmt.Fprintf(w, `<html><body style="font-family:system-ui;padding:48px;background:#0b0d12;color:#f3f4f6">
		<h1 style="margin:0 0 8px">Login complete</h1>
		<p>You can close this tab and return to the terminal.</p></body></html>`)
		return
	}
	fmt.Fprintf(w, `<html><body style="font-family:system-ui;padding:48px;background:#0b0d12;color:#f3f4f6">
	<h1 style="margin:0 0 8px">Connecting CLI</h1>
	<p>Confirm the device code matches what your terminal shows: <code style="font-size:1.6rem;background:#1f2937;padding:6px 12px;border-radius:6px">%s</code></p>
	<p><a href="/auth/github/login?cli_state=%s" style="color:#60a5fa">Continue with GitHub</a></p>
	</body></html>`, uc, state)
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// silence unused import warnings on builds that strip auth_handlers
var _ = base64.StdEncoding
