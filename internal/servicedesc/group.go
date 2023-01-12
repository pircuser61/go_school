package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/trace"
)

const (
	getApproversGroup = "/api/chainsmith/v1/approver/"
	getExecutorsGroup = "/api/chainsmith/v1/executors/"
)

type Approver struct {
	Login string `json:"login"`
}

type Executor struct {
	Login string `json:"login"`
}

type ApproversGroup struct {
	GroupID   string     `json:"groupID"`
	GroupName string     `json:"groupName"`
	People    []Approver `json:"people"`
}

type ExecutorsGroup struct {
	GroupID   string     `json:"groupID"`
	GroupName string     `json:"groupName"`
	People    []Executor `json:"people"`
}

//nolint:dupl //its not duplicate
func (s *Service) GetApproversGroup(ctx context.Context, groupID string) (*ApproversGroup, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_approvers_group")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.SdURL, getApproversGroup, groupID)

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.Cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	res := &ApproversGroup{}
	if unmErr := json.NewDecoder(resp.Body).Decode(res); unmErr != nil {
		return nil, unmErr
	}

	log := logger.GetLogger(ctx)
	log.Info(fmt.Sprintf("got %d from group: %s", len(res.People), res.GroupName))

	return res, nil
}

//nolint:dupl //its not duplicate
func (s *Service) GetExecutorsGroup(ctx context.Context, groupID string) (*ExecutorsGroup, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_executors_group")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.SdURL, getExecutorsGroup, groupID)

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.Cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	res := &ExecutorsGroup{}
	if unmErr := json.NewDecoder(resp.Body).Decode(res); unmErr != nil {
		return nil, unmErr
	}

	log := logger.GetLogger(ctx)
	log.Info(fmt.Sprintf("got %d from group: %s", len(res.People), res.GroupName))

	return res, nil
}
