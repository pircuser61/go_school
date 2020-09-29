package handlers

import (
	"net/http"

	"go.opencensus.io/trace"
)

func (ae APIEnv) GetTags(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(req.Context(), "get_tags")
	defer s.End()

	err := Teapot.sendError(w)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}

func (ae APIEnv) CreateTag(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(req.Context(), "create_tag")
	defer s.End()

	err := Teapot.sendError(w)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}

func (ae APIEnv) EditTag(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(req.Context(), "edit_tag")
	defer s.End()

	err := Teapot.sendError(w)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}

func (ae APIEnv) RemoveTag(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(req.Context(), "remove_tag")
	defer s.End()

	err := Teapot.sendError(w)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}
