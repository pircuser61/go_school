package api

import (
	"github.com/pkg/errors"
)

const (
	defaultLimit  = 10
	defaultOffset = 0
)

func parseLimitOffsetWithDefault(limit, offset *int) (lim, off int) {
	lim, off = defaultLimit, defaultOffset
	if limit != nil {
		lim = *limit
	}

	if offset != nil {
		off = *offset
	}

	return lim, off
}

//nolint:gocritic // params без поинтера нужен для интерфейса
func convertTaskUserParams(up GetTasksUsersParams) (*GetTasksParams, error) {
	var selectAs string

	if up.SelectAs != nil {
		selectAs = string(*up.SelectAs)

		valid := selectAsValid(selectAs)
		if !valid {
			return nil, errors.New("invalid value in SelectAs filter")
		}
	}

	selectAsParams := GetTasksParamsSelectAs(selectAs)

	return &GetTasksParams{
		Name:               up.Name,
		TaskIDs:            up.TaskIDs,
		Order:              up.Order,
		OrderBy:            up.OrderBy,
		Limit:              up.Limit,
		Offset:             up.Offset,
		Created:            up.Created,
		Archived:           up.Archived,
		SelectAs:           &selectAsParams,
		ForCarousel:        up.ForCarousel,
		Status:             up.Status,
		Receiver:           up.Receiver,
		HasAttachments:     up.HasAttachments,
		Initiator:          up.Initiator,
		InitiatorLogins:    up.InitiatorLogins,
		ProcessingLogins:   up.ProcessingLogins,
		ProcessingGroupIds: up.ProcessingGroupIds,
		ExecutorLogins:     up.ExecutorLogins,
		ExecutorGroupIds:   up.ExecutorGroupIds,
	}, nil
}

//nolint:gocritic // params без поинтера нужен для интерфейса
func convertParamsTaskToSchema(up GetTasksSchemasParams) (*GetTasksParams, error) {
	var selectAs string

	if up.SelectAs != nil {
		selectAs = string(*up.SelectAs)

		valid := selectAsValid(selectAs)
		if !valid {
			return nil, errors.New("invalid value in SelectAs filter")
		}
	}

	selectAsParams := GetTasksParamsSelectAs(selectAs)

	return &GetTasksParams{
		Name:               up.Name,
		TaskIDs:            up.TaskIDs,
		Order:              up.Order,
		OrderBy:            up.OrderBy,
		Limit:              up.Limit,
		Offset:             up.Offset,
		Created:            up.Created,
		Archived:           up.Archived,
		SelectAs:           &selectAsParams,
		ForCarousel:        up.ForCarousel,
		Status:             up.Status,
		Receiver:           up.Receiver,
		HasAttachments:     up.HasAttachments,
		Initiator:          up.Initiator,
		InitiatorLogins:    up.InitiatorLogins,
		ProcessingLogins:   up.ProcessingLogins,
		ProcessingGroupIds: up.ProcessingGroupIds,
	}, nil
}
