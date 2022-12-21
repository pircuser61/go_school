package human_tasks

import "time"

type Delegations []Delegation

type Delegation struct {
	FromDate        time.Time
	ToDate          time.Time
	FromLogin       string
	ToLogin         string
	DelegationTypes []DelegationType
}

type DelegationLogins map[string]time.Time

type DelegationType string

const (
	ApprovementDelegationType DelegationType = "approvement"
	ExecutionDelegationType   DelegationType = "execution"
)

func (delegations *Delegations) FindDelegationsFor(login string, delegationType DelegationType) DelegationLogins {
	var loginsAndDates DelegationLogins

	for _, d := range *delegations {
		var neededDelegationTypeExist = d.checkForDelegationType(delegationType)

		if neededDelegationTypeExist {
			if currDate, ok := loginsAndDates[login]; ok {
				if currDate.Before(d.ToDate) /* currDate < d.ToDate */ {
					loginsAndDates[login] = d.ToDate
				}
			} else {
				loginsAndDates[login] = d.ToDate
			}
		}
	}

	return loginsAndDates
}

func (d *Delegation) checkForDelegationType(delegationType DelegationType) bool {
	for _, dt := range d.DelegationTypes {
		if delegationType == dt {
			return true
		}
	}
	return false
}
