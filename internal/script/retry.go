package script

import (
	"context"
	"net/http"
)

type retryCnt struct{}

func MakeContextWithRetryCnt(ctx context.Context) context.Context {
	count := 0
	return context.WithValue(ctx, retryCnt{}, &count)
}

func IncreaseReqRetryCntREST(req *http.Request) {
	cnt := req.Context().Value(retryCnt{})
	i, _ := cnt.(*int)
	if i != nil {
		*i++
	}
}

func IncreaseReqRetryCntGRPC(ctx context.Context) {
	cnt := ctx.Value(retryCnt{})
	i, _ := cnt.(*int)
	if i != nil {
		*i++
	}
}

func GetRetryCnt(ctx context.Context) int {
	attempt, _ := ctx.Value(retryCnt{}).(*int)
	if attempt == nil {
		return 0
	}

	return *attempt
}
