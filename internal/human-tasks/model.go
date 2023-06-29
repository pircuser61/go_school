package human_tasks

import (
	"time"

	"golang.org/x/exp/slices"
)

type Delegations []Delegation

type Delegation struct {
	FromDate        time.Time
	ToDate          time.Time
	DelegationTypes []string
	FromLogin       string // Delegator
	ToLogin         string // Delegate
}

type DelegationLogins map[string]Delegation

func (delegations *Delegations) GetUniqueLogins() []string {
	var uniqueLogins = make(map[string]struct{}, 0)

	for _, d := range *delegations {
		uniqueLogins[d.FromLogin] = struct{}{}
	}

	logins := make([]string, 0, len(uniqueLogins))
	for k := range uniqueLogins {
		logins = append(logins, k)
	}
	return logins
}

func (delegations *Delegations) FilterByType(delegationType string) Delegations {
	filteredDelegations := make([]Delegation, 0)
	for _, delegation := range *delegations {
		if slices.Contains(delegation.DelegationTypes, delegationType) {
			filteredDelegations = append(filteredDelegations, delegation)
		}
	}
	return filteredDelegations
}

func (delegations *Delegations) GetUserInArrayWithDelegations(logins []string) (result []string) {
	if delegations == nil {
		return logins
	}

	var uniqueLogins = make(map[string]struct{}, 0)

	for _, login := range logins {
		uniqueLogins[login] = struct{}{}

		for _, d := range *delegations {
			if d.FromLogin == login {
				if _, ok := uniqueLogins[d.ToLogin]; !ok {
					uniqueLogins[d.ToLogin] = struct{}{}
				}
			}
		}

		for k := range uniqueLogins {
			result = append(result, k)
		}
	}

	return result
}

func (delegations *Delegations) GetUserInArrayWithDelegators(logins []string) (result []string) {
	var uniqueLogins = make(map[string]struct{}, 0)

	for _, login := range logins {
		uniqueLogins[login] = struct{}{}

		for _, d := range *delegations {
			if d.ToLogin == login {
				if _, ok := uniqueLogins[d.FromLogin]; !ok {
					uniqueLogins[d.FromLogin] = struct{}{}
				}
			}
		}

		for k := range uniqueLogins {
			result = append(result, k)
		}
	}

	return result
}

func (delegations *Delegations) FindDelegationsTo(login string) Delegations {
	var loginsAndDates = make(map[string]Delegation, 0)
	var result = make([]Delegation, 0)

	for _, dd := range *delegations {
		if dd.ToLogin == login {
			if exist, ok := loginsAndDates[dd.FromLogin]; ok {
				var currDate = exist.ToDate
				if currDate.Before(dd.ToDate) {
					loginsAndDates[dd.FromLogin] = Delegation{
						FromLogin: dd.FromLogin,
						ToLogin:   dd.ToLogin,
						FromDate:  dd.FromDate,
						ToDate:    dd.ToDate,
					}
				}
			} else {
				loginsAndDates[dd.FromLogin] = Delegation{
					FromLogin: dd.FromLogin,
					ToLogin:   dd.ToLogin,
					FromDate:  dd.FromDate,
					ToDate:    dd.ToDate,
				}
			}
		}
	}

	for _, v := range loginsAndDates {
		result = append(result, v)
	}

	return result
}

func (delegations *Delegations) FindDelegatorFor(login string, entries []string) (result string, ok bool) {
	for _, entry := range entries {
		for _, delegator := range delegations.GetDelegators(login) {
			if delegator == entry {
				result = delegator
				return result, true
			}
		}
	}

	return "", false
}

func (delegations *Delegations) GetDelegators(login string) []string {
	var result = make([]string, 0)
	if len(*delegations) == 0 {
		return result
	}

	for _, delegation := range *delegations {
		if login == delegation.ToLogin {
			result = append(result, delegation.FromLogin)
		}
	}

	return result
}

func (delegations *Delegations) GetDelegates(login string) []string {
	var result = make([]string, 0)
	if len(*delegations) == 0 {
		return result
	}

	for _, delegation := range *delegations {
		if login == delegation.FromLogin {
			result = append(result, delegation.ToLogin)
		}
	}

	return result
}

func (delegations *Delegations) IsLoginDelegateFor(delegate, sourceMember string) bool {
	for _, delegation := range *delegations {
		if delegation.FromLogin == sourceMember {
			if delegation.ToLogin == delegate {
				return true
			}
		}
	}
	return false
}
