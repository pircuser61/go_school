package entity

type ApproveActionName struct {
	Id    string
	Title string
}

type ApproveStatus struct {
	Id    string
	Title string
}

type NodeDecision struct {
	Decision string
	Title    string
	Id       string
	NodeType string
}
