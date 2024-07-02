package servicedesc

type GroupMember struct {
	Login string `json:"login"`
}

type WorkGroup struct {
	GroupID   string        `json:"groupID"`
	GroupName string        `json:"groupName"`
	People    []GroupMember `json:"people"`
}
