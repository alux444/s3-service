package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"s3-service/internal/auth"
	"s3-service/internal/database"
)

type AuditEventRecorder interface {
	RecordEvent(ctx context.Context, event database.AuditEventWrite) error
}

func AuditEventsMiddleware(logger *slog.Logger, recorder AuditEventRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("audit_capture_started",
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", chimiddleware.GetReqID(r.Context()),
			)
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			outcome := "error"
			switch {
			case status >= 200 && status < 400:
				outcome = "allowed"
			case status == http.StatusUnauthorized || status == http.StatusForbidden:
				outcome = "denied"
			}

			claims, ok := ClaimsFromContext(r.Context())
			actorType := "system"
			actorID := ""
			projectID := ""
			appID := ""
			if ok {
				actorType = principalTypeToActorType(claims.PrincipalType)
				actorID = claims.Subject
				projectID = claims.ProjectID
				appID = claims.AppID
			}

			event := database.AuditEventWrite{
				RequestID:  chimiddleware.GetReqID(r.Context()),
				EventType:  "api_request",
				ActorType:  actorType,
				ActorID:    actorID,
				ProjectID:  projectID,
				AppID:      appID,
				Action:     deriveAction(r),
				Outcome:    outcome,
				HTTPStatus: status,
				Metadata: map[string]any{
					"method": r.Method,
					"path":   r.URL.Path,
				},
			}

			if err := recorder.RecordEvent(r.Context(), event); err != nil {
				logger.Warn("failed to record audit event", "error", err, "path", r.URL.Path)
				logger.Info("audit_capture_failed",
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", event.RequestID,
					"http_status", status,
					"outcome", outcome,
				)
				return
			}
			logger.Info("audit_capture_completed",
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", event.RequestID,
				"http_status", status,
				"outcome", outcome,
				"actor_type", actorType,
				"actor_id", actorID,
				"project_id", projectID,
				"app_id", appID,
				"action", event.Action,
			)
		})
	}
}

func deriveAction(r *http.Request) string {
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1/")
	if trimmed == r.URL.Path || trimmed == "" {
		return strings.ToLower(r.Method)
	}
	return strings.ReplaceAll(trimmed, "/", ".")
}

func principalTypeToActorType(pt auth.PrincipalType) string {
	switch pt {
	case auth.PrincipalTypeUser:
		return "user"
	case auth.PrincipalTypeService:
		return "service"
	default:
		return "system"
	}
}
