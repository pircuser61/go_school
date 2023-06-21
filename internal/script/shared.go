package script

type AccessType string

type FormAccessibility struct {
	NodeId      string     `json:"node_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	AccessType  AccessType `json:"accessType"`
}

type TaskSolveTime struct {
	MeanWorkHours float64 `json:"meanWorkHours"`
}
