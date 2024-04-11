package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"go.opencensus.io/trace"
)

const (
	getSchemaByID          = "/api/herald/v1/schema/"
	getSchemaByBlueprintID = "/api/herald/v1/schema/"
	getUserInfo            = "/api/herald/v1/externalData/user/single?search=%s"
	getWorkGroup           = "/api/chainsmith/v1/workGroup/"
)

type Service struct {
	SdURL string
	Cli   *http.Client
	Cache cachekit.Cache
}

type GroupMember struct {
	Login string `json:"login"`
}

type WorkGroup struct {
	GroupID   string        `json:"groupID"`
	GroupName string        `json:"groupName"`
	People    []GroupMember `json:"people"`
}

type SsoPerson struct {
	Fullname    string `json:"fullname"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Mobile      string `json:"mobile"`
	FullOrgUnit string `json:"fullOrgUnit"`
	Position    string `json:"position"`
	Phone       string `json:"phone"`
	Tabnum      string `json:"tabnum"`
}

func (s *Service) GetSsoPerson(ctx context.Context, username string) (*SsoPerson, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_sso_person")
	defer span.End()

	if sso.IsServiceUserName(username) {
		return &SsoPerson{
			Username: username,
		}, nil
	}

	reqURL := fmt.Sprintf("%s%s", s.SdURL, fmt.Sprintf(getUserInfo, username))

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
		return nil, fmt.Errorf("bad status code from sso: %d, username: %s", resp.StatusCode, username)
	}

	res := &SsoPerson{}
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}

	return res, nil
}

//nolint:dupl //its not duplicate
func (s *Service) GetWorkGroup(ctx context.Context, groupID string) (*WorkGroup, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_work_group")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.SdURL, getWorkGroup, groupID)

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

	res := &WorkGroup{}
	if unmErr := json.NewDecoder(resp.Body).Decode(res); unmErr != nil {
		return nil, unmErr
	}

	log := logger.GetLogger(ctx)
	log.Info(fmt.Sprintf("got %d from group: %s", len(res.People), res.GroupName))

	return res, nil
}

func (s *Service) GetSchemaByID(ctx context.Context, schemaID string) (map[string]interface{}, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_schema_by_id")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.SdURL, getSchemaByID, schemaID)

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

	schema := make(map[string]interface{})
	if unmErr := json.NewDecoder(resp.Body).Decode(&schema); unmErr != nil {
		return nil, unmErr
	}

	return schema, nil
}

func (s *Service) GetSchemaByBlueprintID(ctx context.Context, blueprintID string) (map[string]interface{}, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_schema_by_blueprint_id")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s%s", s.SdURL, getSchemaByBlueprintID, blueprintID, "/json")

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

	schema := make(map[string]interface{})
	if unmErr := json.NewDecoder(resp.Body).Decode(&schema); unmErr != nil {
		return nil, unmErr
	}

	return schema, nil
}

func (s *Service) GetSdURL() string {
	return s.SdURL
}

func (s *Service) GetCli() *http.Client {
	return s.Cli
}
