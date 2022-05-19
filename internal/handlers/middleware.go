package handlers

import (
	"context"
	"net/http"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const XRequestIDHeader = "X-Request-Id"

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

type userInfoCtx struct{}

func GetUserInfoFromCtx(ctx context.Context) (*sso.UserInfo, error) {
	uii := ctx.Value(userInfoCtx{})
	if uii == nil {
		return nil, errors.New("can't find userinfo in context")
	}

	ui, ok := uii.(*sso.UserInfo)
	if !ok {
		return nil, errors.New("not userinfo in context")
	}

	return ui, nil
}

func SetUserInfoToCtx(ctx context.Context, ui *sso.UserInfo) context.Context {
	return context.WithValue(ctx, userInfoCtx{}, ui)
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

			ctxUI := SetUserInfoToCtx(ctx, ui)
			rUI := r.WithContext(ctxUI)

			next.ServeHTTP(w, rUI)
		})
	}
}
