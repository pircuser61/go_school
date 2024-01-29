package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

func (gb *GoExecutionBlock) typedAuthor(ctx context.Context) (*sso.UserInfo, error) {
	author, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.UpdateData.ByLogin)
	if err != nil {
		return nil, err
	}

	typedAuthor, err := author.ToUserinfo()
	if err != nil {
		return nil, err
	}

	return typedAuthor, nil
}
