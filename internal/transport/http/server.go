package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/mux"

	"github.com/pircuser61/go_school/config"
	"github.com/pircuser61/go_school/internal/models"
	"github.com/pircuser61/go_school/internal/storage"
)

var l *slog.Logger
var store storage.MateralStore

func Run(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, materialStore storage.MateralStore) {
	l = logger
	store = materialStore

	port := config.GetHttpPort()
	l.Info("запуск http сервера", slog.String("адрес", port))
	/*
		wrap := func( f func(http.ResponseWriter,*http.Request,  l *slog.Logger, s storage.MateralStore)) func(http.ResponseWriter,*http.Request){
			return func(rw http.ResponseWriter,req *http.Request){ f(rw, req, l, s)}
		}
	*/
	router := mux.NewRouter()
	router.HandleFunc("/materials", materials).Methods("GET")

	srv := http.Server{Addr: port, Handler: router}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			l.Error("http сервер завершил работу с ошибкой", slog.String("", err.Error()))
		} else {
			l.Info("http сервер: остановлен прием запросов")
		}
	}()
	<-ctx.Done()

	l.Info("Остановка http сервера...")
	err := srv.Shutdown(context.TODO())
	if err != nil {
		l.Error("Ошибка во время остановки http сервера", slog.String("", err.Error()))
	} else {
		l.Info("http сервер завершил работу")
	}
}

func materials(rw http.ResponseWriter, req *http.Request) {
	l.Debug("Запрос списка материалов")
	filter := models.MaterialListFilter{}
	data, err := store.Materials(req.Context(), filter)
	if err != nil {
		// todo добавить метод write response
	}
	err = json.NewEncoder(rw).Encode(data)
	if err != nil {
		// todo добавить метод write response
	}
}
