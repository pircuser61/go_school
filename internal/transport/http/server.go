package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"

	"github.com/pircuser61/go_school/config"
	"github.com/pircuser61/go_school/internal/models"
	"github.com/pircuser61/go_school/internal/storage"
)

var l *slog.Logger
var store storage.MateralStore

type response struct {
	Error  bool
	ErrMsg string
	Body   any
}

func Run(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, materialStore storage.MateralStore) {
	l = logger
	store = materialStore

	port := config.GetHttpPort()
	l.Info("http:запуск сервера", slog.String("адрес", port))
	/*
		wrap := func( f func(http.ResponseWriter,*http.Request,  l *slog.Logger, s storage.MateralStore)) func(http.ResponseWriter,*http.Request){
			return func(rw http.ResponseWriter,req *http.Request){ f(rw, req, l, s)}
		}
	*/
	router := mux.NewRouter()
	router.HandleFunc("/materials", materials).Methods("GET")
	router.HandleFunc("/materials", materialCreate).Methods("POST")
	router.HandleFunc("/materials/{id}", materialGet).Methods("GET")
	router.HandleFunc("/materials/{id}", materialUpdate).Methods("PUT")
	router.HandleFunc("/materials/{id}", materialDelete).Methods("DELETE")
	srv := http.Server{Addr: port, Handler: router}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			l.Error("http: сервер завершил работу с ошибкой", slog.String("", err.Error()))
		} else {
			l.Info("http: остановлен прием запросов")
		}
	}()
	<-ctx.Done()

	l.Info("http:Остановка сервера...")
	err := srv.Shutdown(context.TODO())
	if err != nil {
		l.Error("http:Ошибка во время остановки сервера", slog.String("", err.Error()))
	} else {
		l.Info("http:сервер завершил работу")
	}
}

func materialCreate(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	l.Debug("http:создание материала")
	var m models.Material
	err := json.NewDecoder(req.Body).Decode(&m)
	if err != nil {
		makeResp(rw, nil, err)
		return
	}
	id, err := store.MaterialCreate(req.Context(), m)
	makeResp(rw, id, err)
}

func materialGet(rw http.ResponseWriter, req *http.Request) {
	l.Debug("http:получение материала")
	vars := mux.Vars(req)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		makeResp(rw, nil, err)
		return
	}
	m, err := store.MaterialGet(req.Context(), id)
	makeResp(rw, m, err)
}

func materialUpdate(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	l.Debug("http:обновление материала")
	vars := mux.Vars(req)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		makeResp(rw, nil, err)
		return
	}
	var m models.Material
	err = json.NewDecoder(req.Body).Decode(&m)
	if err == nil {
		m.UUID = id
		err = store.MaterialUpdate(req.Context(), m)
	}
	makeResp(rw, nil, err)
}

func materialDelete(rw http.ResponseWriter, req *http.Request) {
	l.Debug("http:удаление материала")
	defer req.Body.Close()
	vars := mux.Vars(req)
	id, err := uuid.Parse(vars["id"])
	if err == nil {
		err = store.MaterialDelete(req.Context(), id)
	}
	makeResp(rw, nil, err)
}

func materials(rw http.ResponseWriter, req *http.Request) {
	l.Debug("http:список материалов", req.URL.Query())
	defer req.Body.Close()
	var filter models.MaterialListFilter

	/*
		limit := req.FormValue("limit")
		if limit > "" {
			filter.Limit, err = strconv.ParseUint(limit, 10, 64)
		}
	*/
	decoder := schema.NewDecoder()
	err := decoder.Decode(&filter, req.URL.Query())
	if err != nil {
		makeResp(rw, nil, err)
		return
	}

	data, err := store.Materials(req.Context(), filter)
	makeResp(rw, data, err)
}

func makeResp(rw http.ResponseWriter, body any, err error) {
	/*
		rw.Header().Set("Access-Control-Allow-Origin", "*")

		rw.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		rw.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	*/
	rw.Header().Set("Content-Type", "application/json")
	if err == nil {
		err = json.NewEncoder(rw).Encode(response{Body: body})
	} else {
		err = json.NewEncoder(rw).Encode(response{Error: true, ErrMsg: err.Error()})
	}
	if err != nil {
		l.Error("http: jsonEncoder", slog.String("", err.Error()))
		fmt.Fprintf(rw, `{"Error": true, "ErrMsg", %s}`, err.Error())
	}
}
