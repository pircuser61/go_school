package pipeline

import (
	"context"

	"github.com/pkg/errors"
)

func (gb *GoApproverBlock) replyAddInfo(ctx context.Context, id string, updateParams *requestInfoParams) (string, error) {
	var (
		initiator    = gb.RunContext.Initiator
		currentLogin = gb.RunContext.UpdateData.ByLogin
	)

	if len(gb.State.AddInfo) == 0 {
		return "", errors.New("don't answer after request")
	}

	if currentLogin != initiator {
		return "", NewUserIsNotPartOfProcessErr()
	}

	if updateParams.LinkID == nil {
		return "", errors.New("linkId is null when reply")
	}

	parentEntry := gb.State.findAddInfoLogEntry(*updateParams.LinkID)
	if parentEntry == nil || parentEntry.Type == ReplyAddInfoType ||
		gb.State.addInfoLogEntryHasResponse(*updateParams.LinkID) {
		return "", errors.New("bad linkId to submit an answer")
	}

	approverLogin, linkErr := setLinkIDRequest(id, *updateParams.LinkID, gb.State.AddInfo)
	if linkErr != nil {
		return "", linkErr
	}

	err := gb.notifyNewInfoReceived(ctx, approverLogin)
	if err != nil {
		return "", err
	}

	return *updateParams.LinkID, nil
}
