package ctx

import (
	"context"
	"time"
)

func Context(timeout int) context.Context {
	c, _ := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second) //nolint
	return c
}
