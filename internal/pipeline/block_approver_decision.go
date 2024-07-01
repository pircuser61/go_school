package pipeline

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	en "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

//nolint:all // ok
func (a *ApproverData) SetDecision(login, comment string, ds ApproverDecision, attach []en.Attachment, d ht.Delegations) error {
	if ds == "" {
		return errors.New("missing decision")
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	delegators := d.GetDelegators(login)

	delegateFor := a.delegateFor(delegators)

	_, isApproverExist := a.Approvers[login]

	if !(isApproverExist || len(delegateFor) > 0) && login != AutoApprover {
		return NewUserIsNotPartOfProcessErr()
	}

	allOfApprovementRequired := a.ApprovementRule == script.AllOfApprovementRequired

	if !allOfApprovementRequired {
		a.Decision = &ds
		a.Comment = &comment
		a.ActualApprover = &login
		a.DecisionAttachments = attach

		logEntry := ApproverLogEntry{
			Login:       login,
			Decision:    ds,
			Comment:     comment,
			Attachments: attach,
			CreatedAt:   time.Now(),
			LogType:     ApproverLogDecision,
		}
		if len(delegateFor) > 0 && !isApproverExist {
			logEntry.DelegateFor = delegateFor[0]
		}

		a.ApproverLog = append(a.ApproverLog, logEntry)
	}

	if allOfApprovementRequired {
		if a.isUserDecisionSet(login) {
			return fmt.Errorf("decision of user %s is already set", login)
		}

		var (
			overallDecision ApproverDecision
			isFinal         bool
		)

		isAutoApprover := login == AutoApprover

		if isAutoApprover {
			a.ApproverLog = append(
				a.ApproverLog,
				ApproverLogEntry{
					Login:       AutoApprover,
					Decision:    ds,
					Comment:     comment,
					Attachments: attach,
					CreatedAt:   time.Now(),
					LogType:     ApproverLogDecision,
				},
			)

			overallDecision = ds
			isFinal = true
		}

		if !isAutoApprover {
			if isApproverExist {
				a.ApproverLog = append(
					a.ApproverLog,
					ApproverLogEntry{
						Login:       login,
						Decision:    ds,
						Comment:     comment,
						Attachments: attach,
						CreatedAt:   time.Now(),
						LogType:     ApproverLogDecision,
					},
				)
			}

			for _, dl := range delegateFor {
				a.ApproverLog = append(a.ApproverLog, ApproverLogEntry{
					Login:       login,
					Decision:    ds,
					Comment:     comment,
					Attachments: attach,
					CreatedAt:   time.Now(),
					LogType:     ApproverLogDecision,
					DelegateFor: dl,
				})
			}

			overallDecision, isFinal = a.getFinalGroupDecision(ds)
		}

		if !isFinal {
			return nil
		}

		a.Decision = &overallDecision
		a.Comment = &comment
		a.ActualApprover = &login
		a.DecisionAttachments = []en.Attachment{}

		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for _, l := range a.ApproverLog {
			if l.LogType == ApproverLogDecision {
				a.DecisionAttachments = append(a.DecisionAttachments, l.Attachments...)
			}
		}
	}

	return nil
}

//nolint:gocyclo //its ok here
func (a *ApproverData) SetDecisionByAdditionalApprover(login string,
	params additionalApproverUpdateParams, delegations ht.Delegations,
) ([]string, error) {
	checkForAdditionalApprover := func(login string) bool {
		for _, approver := range a.AdditionalApprovers {
			if login == approver.ApproverLogin {
				return true
			}
		}

		return false
	}

	approverFound := checkForAdditionalApprover(login)

	delegateFor, isDelegate := delegations.FindDelegatorFor(login, a.getAdditionalApproversSlice())
	if !(approverFound || isDelegate) {
		return nil, NewUserIsNotPartOfProcessErr()
	}

	if a.Decision != nil {
		return nil, errors.New("decision already set")
	}

	loginsToNotify := make([]string, 0)
	couldUpdateOne := false
	timeNow := time.Now()

	for i := range a.AdditionalApprovers {
		var (
			additionalApprover              = a.AdditionalApprovers[i].ApproverLogin
			isDelegateForAdditionalApprover = delegations.IsLoginDelegateFor(login, additionalApprover)
		)

		if (login != additionalApprover && !isDelegateForAdditionalApprover) ||
			a.AdditionalApprovers[i].Decision != nil {
			continue
		}

		a.AdditionalApprovers[i].Decision = &params.Decision
		a.AdditionalApprovers[i].Comment = &params.Comment
		a.AdditionalApprovers[i].Attachments = params.Attachments

		if a.AdditionalApprovers[i].DecisionTime == nil {
			a.AdditionalApprovers[i].DecisionTime = &timeNow
		}

		if approverFound {
			delegateFor = ""
		}

		approverLogEntry := ApproverLogEntry{
			Login:       login,
			Decision:    params.Decision,
			Comment:     params.Comment,
			Attachments: params.Attachments,
			CreatedAt:   time.Now(),
			LogType:     AdditionalApproverLogDecision,
			DelegateFor: delegateFor,
		}

		a.ApproverLog = append(a.ApproverLog, approverLogEntry)
		loginsToNotify = append(loginsToNotify, a.AdditionalApprovers[i].BaseApproverLogin)
		couldUpdateOne = true
	}

	if !couldUpdateOne {
		return nil, fmt.Errorf("can't approve any request")
	}

	return loginsToNotify, nil
}

func (a *ApproverData) calculateDecisions() (isFinal, rejectExist, sendEditExist bool, p map[ApproverDecision]int) {
	var total int
	positiveDecisions := make(map[ApproverDecision]int)

	for i := range a.ApproverLog {
		log := a.ApproverLog[i]
		if log.LogType != ApproverLogDecision {
			continue
		}

		total++

		if log.Decision != ApproverDecisionRejected && log.Decision != ApproverDecisionSentToEdit {
			count, decisionExists := positiveDecisions[log.Decision]
			if !decisionExists {
				count = 0
			}

			positiveDecisions[log.Decision] = count + 1
		}

		if log.Decision == ApproverDecisionRejected {
			rejectExist = true
		}

		if log.Decision == ApproverDecisionSentToEdit {
			sendEditExist = true
		}
	}

	return total == len(a.Approvers), rejectExist, sendEditExist, positiveDecisions
}

func (a *ApproverData) getFinalGroupDecision(ds ApproverDecision) (finalDecision ApproverDecision, isFinal bool) {
	if ds == ApproverDecisionRejected && !a.WaitAllDecisions {
		return ApproverDecisionRejected, true
	}

	isFinal, isRejectExist, isSendEditExist, positives := a.calculateDecisions()

	if !isFinal {
		return "", isFinal
	}

	if a.WaitAllDecisions && isFinal && isRejectExist {
		return ApproverDecisionRejected, isFinal
	}

	if a.WaitAllDecisions && isFinal && isSendEditExist {
		return ApproverDecisionSentToEdit, isFinal
	}

	maxCount := 0
	for decision, count := range positives {
		if count > maxCount {
			maxCount = count
			finalDecision = decision
		}
	}

	return finalDecision, isFinal
}

func (a *ApproverData) isUserDecisionSet(login string) bool {
	for i := range a.ApproverLog {
		if a.ApproverLog[i].Login == login && a.ApproverLog[i].LogType == ApproverLogDecision {
			return true
		}
	}

	return false
}

func isApproverDecisionExists(login string, logs *[]ApproverLogEntry) bool {
	for i := 0; i < len(*logs); i++ {
		logEntry := (*logs)[i]
		if (logEntry.Login == login || logEntry.DelegateFor == login) && logEntry.LogType == ApproverLogDecision {
			return true
		}
	}

	return false
}
