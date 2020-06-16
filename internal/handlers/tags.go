package handlers

import (
	"context"
	"net/http"

	"go.opencensus.io/trace"
)

func (ae APIEnv) GetTags(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "get_tags")
	defer s.End()

	err := Teapot.sendError(w)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}

func (ae APIEnv) CreateTag(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "create_tag")
	defer s.End()

	err := Teapot.sendError(w)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}

func (ae APIEnv) EditTag(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "edit_tag")
	defer s.End()

	err := Teapot.sendError(w)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}

func (ae APIEnv) RemoveTag(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "remove_tag")
	defer s.End()

	err := Teapot.sendError(w)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}
