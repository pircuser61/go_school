package kafka

import (
	"context"
	"encoding/json"
)

const (
	RunnerInMsgPrefix = "function-result:"
	RunTaskMsgPrefix  = "run-task:"
)

func (s *Service) GetRunnerInMsg(ctx context.Context, key string) (RunnerInMessage, error) {
	msg := RunnerInMessage{}

	data, getErr := s.cache.GetValue(ctx, RunnerInMsgPrefix+key)
	if getErr != nil {
		return msg, getErr
	}

	d := []byte(data.(string))

	if jsonErr := json.Unmarshal(d, &msg); jsonErr != nil {
		return msg, jsonErr
	}

	return msg, nil
}

func (s *Service) SetRunnerInMsg(ctx context.Context, key string, msg RunnerInMessage) error {
	data, jsonErr := json.Marshal(msg)
	if jsonErr != nil {
		return jsonErr
	}

	if setErr := s.cache.SetValue(ctx, RunnerInMsgPrefix+key, data); setErr != nil {
		return setErr
	}

	return nil
}

func (s *Service) DelRunnerInMsg(ctx context.Context, key string) error {
	if err := s.cache.DeleteValue(ctx, RunnerInMsgPrefix+key); err != nil {
		return err
	}

	return nil
}

func (s *Service) GetRunTaskMsg(ctx context.Context, key string) (RunTaskMessage, error) {
	msg := RunTaskMessage{}

	data, getErr := s.cache.GetValue(ctx, RunTaskMsgPrefix+key)
	if getErr != nil {
		return msg, getErr
	}

	d := []byte(data.(string))

	if jsonErr := json.Unmarshal(d, &msg); jsonErr != nil {
		return msg, jsonErr
	}

	return msg, nil
}

func (s *Service) SetRunTaskMsg(ctx context.Context, key string, msg RunTaskMessage) error {
	data, jsonErr := json.Marshal(msg)
	if jsonErr != nil {
		return jsonErr
	}

	if setErr := s.cache.SetValue(ctx, RunTaskMsgPrefix+key, data); setErr != nil {
		return setErr
	}

	return nil
}

func (s *Service) DelRunTaskMsg(ctx context.Context, key string) error {
	if err := s.cache.DeleteValue(ctx, RunTaskMsgPrefix+key); err != nil {
		return err
	}

	return nil
}

func (s *Service) GetCachedKeys(ctx context.Context, prefix string) ([]string, error) {
	keys, err := s.cache.GetAllKeys(ctx, prefix)
	if err != nil {
		return keys, err
	}

	return keys, nil
}
