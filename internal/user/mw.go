package user

import (
	"context"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type (
	userInfoCtx        struct{}
	asOtherUserInfoCtx struct{}
)

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

func GetEffectiveUserInfoFromCtx(ctx context.Context) (*sso.UserInfo, error) {
	// first check if we use other userinfo
	uii := ctx.Value(asOtherUserInfoCtx{})
	if uii != nil {
		ui, ok := uii.(*sso.UserInfo)
		if !ok {
			return nil, errors.New("not userinfo in context")
		}

		return ui, nil
	}

	uii = ctx.Value(userInfoCtx{})
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
	log := logger.GetLogger(ctx).WithField("requestLogin", ui.Username)

	return logger.WithLogger(context.WithValue(ctx, userInfoCtx{}, ui), log)
}

func SetAsOtherUserInfoToCtx(ctx context.Context, ui *sso.UserInfo) context.Context {
	log := logger.GetLogger(ctx).WithField("X-As-Other", ui.Username)

	return logger.WithLogger(context.WithValue(ctx, asOtherUserInfoCtx{}, ui), log)
}
