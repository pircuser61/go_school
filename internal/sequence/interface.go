package sequence

type Service interface {
	GetWorkNumberFromQueue() (workNumber string, ok, needPrefetch bool)
	AddWorkNumbersToQueue(workNumbers []int)
	Lock()
	Unlock()
	GetPrefetchSize() int
}
