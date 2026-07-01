package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"fde-support/internal/environment"
	"fde-support/internal/manifest"
	"fde-support/internal/runtimecore"
	"fde-support/internal/shared"
	"fde-support/internal/trace"
	"fde-support/internal/w2a"
)

type Server struct {
	httpHandler http.Handler
}

func NewServer(m *manifest.SolutionManifest, env environment.ResolvedEnvironment, executor *runtimecore.Executor, store w2a.SignalIdempotencyStore, traceWriter *trace.FileTraceWriter) *Server {
	router := chi.NewRouter()
	signalRouter := NewSignalRouter(m, env, executor, store, traceWriter, newAuditLogWriter())
	webRoot := resolveWebRoot()
	hasWeb := webRoot != ""


	// Apply CSP middleware globally for defense-in-depth across all routes
	router.Use(cspMiddleware)
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	if hasWeb {
		router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(webRoot, "index.html"))
		})
		router.Get("/web", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/web/", http.StatusMovedPermanently)
		})
		router.Get("/web/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(webRoot, "index.html"))
		})
		router.Handle("/web/*", http.StripPrefix("/web/", cspMiddleware(http.FileServer(http.Dir(webRoot)))))
	}
	router.Get("/api/runtime", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, newRuntimeView(m, env))
	})
	router.Get("/api/traces", func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				writeAppError(w, shared.BadRequest("INVALID_TRACE_LIMIT", "limit", "limit must be an integer"))
				return
			}
			limit = parsed
		}
		records, err := traceWriter.List(r.Context(), limit)
		if err != nil {
			writeAppError(w, shared.Internal("TRACE_LIST_FAILED", "", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, records)
	})
	router.Get("/api/traces/{traceId}", func(w http.ResponseWriter, r *http.Request) {
		traceID := chi.URLParam(r, "traceId")
		record, err := traceWriter.Load(r.Context(), traceID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeAppError(w, shared.NotFound("TRACE_NOT_FOUND", "traceId", "trace not found"))
				return
			}
			writeAppError(w, shared.Internal("TRACE_LOAD_FAILED", "traceId", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, record)
	})
	router.Post("/chat", func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
			writeAppError(w, shared.BadRequest("UNSUPPORTED_MEDIA_TYPE", "Content-Type", "Content-Type must be application/json"))
			return
		}
		var payload map[string]any
		if err := decodeJSON(r, &payload); err != nil {
			writeAppError(w, shared.BadRequest("INVALID_JSON", "", err.Error()))
			return
		}
		response, appErr := signalRouter.HandleChat(r.Context(), payload)
		if appErr != nil {
			writeAppError(w, appErr)
			return
		}
		writeJSON(w, http.StatusOK, response)
	})
	for _, sensor := range m.Perception.Sensors {
		if endpoint, ok := sensor.Config["endpointPath"].(string); ok && endpoint != "" {
			sensorCopy := sensor
			router.Post(endpoint, func(w http.ResponseWriter, r *http.Request) {
				if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
					appErr := shared.BadRequest("UNSUPPORTED_MEDIA_TYPE", "Content-Type", "Content-Type must be application/json")
					_ = signalRouter.writeRejectedTrace(r.Context(), sensorCopy, nil, appErr)
					writeAppError(w, appErr)
					return
				}
				var payload map[string]any
				if err := decodeJSON(r, &payload); err != nil {
					appErr := shared.BadRequest("INVALID_JSON", "", err.Error())
					_ = signalRouter.writeRejectedTrace(r.Context(), sensorCopy, nil, appErr)
					writeAppError(w, appErr)
					return
				}
				response, status, appErr := signalRouter.HandleSignal(r.Context(), sensorCopy, payload, r.Header.Get("Authorization"))
				if appErr != nil {
					if status == 0 {
						status = appErr.HTTPStatus
					}
					if status == 0 {
						status = http.StatusInternalServerError
					}
					writeJSON(w, status, map[string]any{"error": appErr})
					return
				}
				if status == 0 {
					status = http.StatusOK
				}
				writeJSON(w, status, response)
			})
		}
	}

	return &Server{httpHandler: router}
}

func (s *Server) Handler() http.Handler {
	return s.httpHandler
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	normalizeJSONNumbers(target)
	return nil
}

func normalizeJSONNumbers(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeValue(item)
		}
	case *map[string]any:
		for key, item := range *v {
			(*v)[key] = normalizeValue(item)
		}
	case []any:
		for i, item := range v {
			v[i] = normalizeValue(item)
		}
	}
}

func normalizeValue(value any) any {
	switch v := value.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		if f, err := v.Float64(); err == nil {
			return f
		}
		return v.String()
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeValue(item)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = normalizeValue(item)
		}
		return v
	default:
		return value
	}
}

func writeAppError(w http.ResponseWriter, appErr *shared.AppError) {
	if appErr.HTTPStatus == 0 {
		appErr.HTTPStatus = http.StatusInternalServerError
	}
	writeJSON(w, appErr.HTTPStatus, map[string]any{"error": appErr})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func resolveWebRoot() string {
	candidates := []string{}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Dir(filepath.Dir(filepath.Dir(file))))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	for _, start := range candidates {
		if root := findWebRoot(start); root != "" {
			return root
		}
	}
	return ""
}

func findWebRoot(start string) string {
	dir := filepath.Clean(start)
	for {
		candidate := filepath.Join(dir, "web")
		if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}


// cspMiddleware adds Content-Security-Policy header to prevent XSS.
func cspMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use Add instead of Set to avoid overwriting any more specific CSP
		// header that downstream handlers or other middleware may have set.
		if w.Header().Get("Content-Security-Policy") == "" {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		}
		next.ServeHTTP(w, r)
	})
}

// newAuditLogWriter creates a writer for security audit events.
func newAuditLogWriter() io.Writer {
	return os.Stderr
}
