package sequence

import (
	c "context"

	sequence "gitlab.services.mts.ru/jocasta/sequence/pkg/proto/gen/src/sequence/v1"
)

func (s *service) GetWorkNumber(ctx c.Context) (workNumber string, err error) {
	resp, err := s.cli.GetWorkNumber(ctx, &sequence.GetWorkNumberRequest{})
	if err != nil {
		s.log.Error("can`t get work number", err)

		return "", err
	}

	return resp.WorkNumber, nil
}
