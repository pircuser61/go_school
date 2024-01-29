package api

import (
	"strings"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

func handleFilterStatus(filters *entity.TaskFilter) {
	ss := strings.Split(*filters.Status, ",")

	uniqueS := make(map[pipeline.TaskHumanStatus]struct{})
	for _, status := range ss {
		uniqueS[pipeline.TaskHumanStatus(strings.Trim(status, "'"))] = struct{}{}
	}

	//nolint:exhaustive // раз не надо было обрабатывать остальные случаи значит не надо // правильно, не уважаю этот линтер
	for status := range uniqueS {
		switch status {
		case pipeline.StatusRejected:
			uniqueS[pipeline.StatusApprovementRejected] = struct{}{}
		case pipeline.StatusApprovementRejected:
			uniqueS[pipeline.StatusRejected] = struct{}{}
		default:
			continue
		}
	}

	newSS := make([]string, 0, len(uniqueS))

	for status := range uniqueS {
		newSS = append(newSS, "'"+string(status)+"'")
	}

	newStatuses := strings.Join(newSS, ",")
	filters.Status = &newStatuses
}
