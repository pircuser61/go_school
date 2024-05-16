package servicedesc

type GroupMember struct {
	Login string `json:"login"`
}

type WorkGroup struct {
	GroupID   string        `json:"groupID"`
	GroupName string        `json:"groupName"`
	People    []GroupMember `json:"people"`
}

type SsoPerson struct {
	Fullname    string `json:"fullname"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Mobile      string `json:"mobile"`
	FullOrgUnit string `json:"fullOrgUnit"`
	Position    string `json:"position"`
	Phone       string `json:"phone"`
	Tabnum      string `json:"tabnum"`
}
