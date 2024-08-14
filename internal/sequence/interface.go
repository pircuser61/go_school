package sequence

import c "context"

type Service interface {
	GetWorkNumberFromQueue(ctx c.Context) (workNumber string, ok, needPrefetch bool)
	AddWorkNumbersToQueue(workNumbers []int)
	Lock()
	Unlock()
	GetPrefetchSize() int
}
