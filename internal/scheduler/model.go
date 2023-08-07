package scheduler

type CreateTask struct {
	WorkNumber  string
	WorkId      string
	ActionName  string
	WaitSeconds int
}
