package human_tasks

import "time"

type Delegations []Delegation

type Delegation struct {
	FromDate  time.Time
	ToDate    time.Time
	FromLogin string // Delegator
	ToLogin   string // Delegate
}

type DelegationLogins map[string]Delegation

func (delegations *Delegations) GetUserInArrayWithDelegations(logins []string) (result []string) {
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

func (delegations *Delegations) IsLoginDelegateFor(delegate string, sourceMember string) bool {
	for _, delegation := range *delegations {
		if delegation.FromLogin == sourceMember {
			if delegation.ToLogin == delegate {
				return true
			}
		}
	}
	return false
}
