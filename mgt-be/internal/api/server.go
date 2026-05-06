// Package api wires the HTTP layer (chi router, middleware, handlers).
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/santhosh/mgt-be/internal/ai"
	"github.com/santhosh/mgt-be/internal/auth"
	"github.com/santhosh/mgt-be/internal/config"
	"github.com/santhosh/mgt-be/internal/core"
	"github.com/santhosh/mgt-be/internal/crypto"
	"github.com/santhosh/mgt-be/internal/db"
	"github.com/santhosh/mgt-be/internal/search"
)

// Server backs the REST API used by mgt-ui. It pulls every dependency from a
// shared *core.Core so the gRPC server (used by mgt-cli) sees the same
// DeviceFlow / queue notifier / db pool.
type Server struct {
	cfg    *config.Config
	db     *db.DB
	sealer *crypto.Sealer
	signer *auth.SessionSigner
	device *auth.DeviceFlow
	ai     *ai.Client
	search *search.Client
	queue  core.QueueController
}

func NewServer(c *core.Core) *Server {
	return &Server{
		cfg:    c.Cfg,
		db:     c.DB,
		sealer: c.Sealer,
		signer: c.Signer,
		device: c.Device,
		ai:     c.AI,
		search: c.Search,
		queue:  c.Queue,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(loggingMiddleware)
	r.Use(s.corsMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	// OAuth + CLI device flow (unauthenticated).
	r.Route("/auth", func(r chi.Router) {
		r.Get("/github/login", s.githubLogin)
		r.Get("/github/callback", s.githubCallback)
		r.Get("/cli/start", s.cliStart)
		r.Get("/cli/poll", s.cliPoll)
		r.Get("/cli", s.cliConfirmPage)
		r.Post("/logout", s.logout)
	})

	// GitHub webhooks (HMAC-verified, no session).
	r.Post("/webhooks/github", s.githubWebhook)

	// Authenticated API.
	r.Route("/api", func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Get("/me", s.me)
		r.Get("/repos", s.listRepos)
		r.Post("/repos/sync", s.syncRepos)

		r.Route("/repos/{repoID}", func(r chi.Router) {
			r.Get("/stacks", s.listStacks)
			r.Get("/stacks/by-branch", s.getStackByBranch)
			r.Post("/stacks", s.createStack)
			r.Post("/stacks/{stackID}/branches", s.addBranch)
			r.Patch("/stacks/{stackID}/branches/{branchID}", s.moveBranch)
			r.Delete("/stacks/{stackID}/branches/{branchID}", s.deleteBranch)

			r.Get("/pulls", s.listPRs)
			r.Get("/pulls/{number}", s.getPR)
			r.Post("/pulls", s.createPR)
			r.Post("/pulls/{number}/describe", s.describePR)

			r.Get("/merge-queue", s.listMergeQueue)
			r.Post("/merge-queue", s.enqueueMerge)
			r.Delete("/merge-queue/{entryID}", s.cancelMerge)
		})
	})

	return r
}

// ── Helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// loggingMiddleware emits a single structured line per request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"dur_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (origin == s.cfg.UIBaseURL || strings.HasPrefix(origin, "http://localhost")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// repoFromCtx fetches the repo by its URL param and verifies the user can see
// it (everyone authenticated can today; refine when adding sharing).
func (s *Server) repoFromCtx(r *http.Request) (db.Repository, error) {
	idStr := chi.URLParam(r, "repoID")
	id, err := parseInt64(idStr)
	if err != nil {
		return db.Repository{}, fmt.Errorf("bad repo id")
	}
	return s.db.GetRepository(r.Context(), id)
}
