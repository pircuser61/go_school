package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

func (gb *GoExecutionBlock) initiatorInfo(ctx context.Context) (*sso.UserInfo, error) {
	initiator, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
	if err != nil {
		return nil, err
	}

	initiatorInfo, err := initiator.ToUserinfo()
	if err != nil {
		return nil, err
	}

	return initiatorInfo, nil
}
