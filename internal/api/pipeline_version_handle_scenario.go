package api

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

func (ae *Env) handleScenario(ctx context.Context, p *entity.EriusScenario, ui *sso.UserInfo) (err error) {
	switch p.Status {
	case db.StatusApproved:
		err = ae.DB.SwitchApproved(ctx, p.PipelineID, p.VersionID, ui.Username)
		if err != nil {
			return err
		}
	case db.StatusRejected:
		err = ae.DB.SwitchRejected(ctx, p.VersionID, p.CommentRejected, ui.Username)
		if err != nil {
			return err
		}
	}

	return nil
}
