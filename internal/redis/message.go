package redisdb

import (
	"context"
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
)

const (
	RunnerInMsgPrefix = "function-result:"
)

func (db *DB) GetRunnerInMsg(ctx context.Context, key string) (kafka.RunnerInMessage, error) {
	msg := kafka.RunnerInMessage{}

	data, getErr := db.Get(ctx, key).Result()
	if getErr != nil {
		return msg, getErr
	}

	if jsonErr := json.Unmarshal([]byte(data), &msg); jsonErr != nil {
		return msg, jsonErr
	}

	return msg, nil
}

func (db *DB) SetRunnerInMsg(ctx context.Context, key string, msg kafka.RunnerInMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if status := db.Set(ctx, RunnerInMsgPrefix+key, data, db.ttlRunnerInMsg); status.Err() != nil {
		return status.Err()
	}

	return nil
}

func (db *DB) DelRunnerInMsg(ctx context.Context, key string) error {
	if status := db.Del(ctx, RunnerInMsgPrefix+key); status.Err() != nil {
		return status.Err()
	}

	return nil
}
