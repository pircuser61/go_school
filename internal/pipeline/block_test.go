package pipeline

import (
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
	"net/http"
)

func newStoreWithData(data map[string]interface{}) *store.VariableStore {
	s := store.NewStore()

	for key, val := range data {
		s.SetValue(key, val)
	}

	return s
}

func storeContainsData(s *store.VariableStore, data map[string]interface{}) bool {
	st, _ := s.GrabStorage()

	for key, valExp := range data {
		if val, ok := st[key]; !ok || val != valExp {
			return false
		}
	}

	return true
}

var (
	RunOnlyFunction = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(200)
	})

	WithOutputFunction = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("{\"sOutput\":\"sOutputValue\"}"))
	})
)
