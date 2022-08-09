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
	authorizationHeader = "Authorization"
	getGroup            = "/v1/approver/"
)

type Approver struct {
	Login string `json:"login"`
}

type ApproversGroup struct {
	GroupID   string     `json:"groupID"`
	GroupName string     `json:"groupName"`
	People    []Approver `json:"people"`
}

func (s *Service) GetApproversGroup(ctx context.Context, groupID string) (*ApproversGroup, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_approvers_group")
	defer span.End()

	var req *http.Request
	var err error

	reqURL := fmt.Sprintf("%s%s%s", s.chainsmithURL, getGroup, groupID)

	req, err = makeRequest(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.cli.Do(req)
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
