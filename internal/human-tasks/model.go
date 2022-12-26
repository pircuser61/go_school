package human_tasks

import "time"

type Delegations []Delegation

type Delegation struct {
	FromDate  time.Time
	ToDate    time.Time
	FromLogin string
	ToLogin   string
}

type DelegationLogins map[string]Delegation

func (delegations *Delegations) GetUserInArrayWithDelegations(login string) (result []string) {
	var uniqueLogins = make(map[string]interface{}, 0)
	uniqueLogins[login] = login

	for _, d := range *delegations {
		if d.FromLogin == login {
			if _, ok := uniqueLogins[d.ToLogin]; !ok {
				uniqueLogins[d.ToLogin] = d.ToLogin
			}
		}
	}

	for k := range uniqueLogins {
		result = append(result, k)
	}

	return result
}

func (delegations *Delegations) FindDelegationsTo(login string) Delegations {
	var loginsAndDates DelegationLogins
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

func (delegations *Delegations) DelegateFor(login string) string {
	if len(*delegations) == 0 {
		return ""
	}

	for _, delegation := range *delegations {
		if login == delegation.ToLogin {
			return delegation.FromLogin
		}
	}

	return ""
}
