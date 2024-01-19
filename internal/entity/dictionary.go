package entity

type ApproveActionName struct {
	ID    string
	Title string
}

type ApproveStatus struct {
	ID    string
	Title string
}

type NodeDecision struct {
	Decision string
	Title    string
	ID       string
	NodeType string
}
