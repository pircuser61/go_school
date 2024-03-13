package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"go.opencensus.io/trace"
)

const getUserInfo = "/api/herald/v1/externalData/user/single?search=%s"

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
