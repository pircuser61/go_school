package user

import (
	"context"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

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
