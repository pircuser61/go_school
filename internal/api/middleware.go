package api

import (
	"context"
	"net/http"
	"strings"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

const (
	XRequestIDHeader    = "X-Request-Id"
	AuthorizationHeader = "Authorization"
	AsOtherHeader       = "X-As-Other"
)

func RequestIDMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(XRequestIDHeader)
		if reqID == "" {
			reqID = uuid.New().String()
			w.Header().Set(XRequestIDHeader, reqID)
			r.Header.Set(XRequestIDHeader, reqID)
		}

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func LoggerMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return ochttp.Handler{
			Handler: http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				ctxLocal, span := trace.StartSpan(req.Context(), req.Method+" "+req.URL.String())
				defer span.End()

				newLogger := log.WithFields(map[string]interface{}{
					"method": req.Method,
					"url":    req.URL.String(),
					"host":   req.Host,
				})

				newLogger.Info("request")
				ctx := logger.WithLogger(ctxLocal, newLogger)

				next.ServeHTTP(res, req.WithContext(ctx))
			}),
		}.Handler
	}
}

func WithUserInfo(ssoS *sso.Service, log logger.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ui, err := ssoS.GetUserinfo(ctx, r)
			if err != nil {
				e := GetUserinfoErr
				log.Error(e.errorMessage(err))
				_ = e.sendError(w)

				return
			}

			ctxUI := user.SetUserInfoToCtx(ctx, ui)
			rUI := r.WithContext(ctxUI)

			next.ServeHTTP(w, rUI)
		})
	}
}

func WithAsOtherUserInfo(ps *people.Service, log logger.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			username := r.Header.Get(AsOtherHeader)

			if username != "" {
				u, err := ps.GetUser(ctx, strings.ToLower(username))
				if err != nil {
					e := GetUserinfoErr
					log.Error(e.errorMessage(err))
					_ = e.sendError(w)

					return
				}
				ui, err := u.ToUserinfo()
				if err != nil {
					e := GetUserinfoErr
					log.Error(e.errorMessage(err))
					_ = e.sendError(w)

					return
				}

				ctx = user.SetAsOtherUserInfoToCtx(ctx, ui)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func SetAuthTokenInContext(log logger.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			token := r.Header.Get(AuthorizationHeader)
			log.Info("auth token: ", token)

			if token != "" {
				ctx = context.WithValue(ctx, AuthorizationHeader, token)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
