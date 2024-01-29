package pipeline

func (a *ApproverData) delegateFor(delegators []string) []string {
	delegateFor := make([]string, 0)

	for approver := range a.Approvers {
		for _, delegator := range delegators {
			if delegator == approver && !decisionForPersonExists(delegator, &a.ApproverLog) {
				delegateFor = append(delegateFor, delegator)
			}
		}
	}

	return delegateFor
}
