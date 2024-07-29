package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	pgStorage "github.com/pircuser61/go_school/internal/storage/postgress"
	httpServer "github.com/pircuser61/go_school/internal/transport/http"
)

func main() {
	opts := slog.HandlerOptions{Level: slog.LevelDebug}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &opts))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	wg := sync.WaitGroup{}

	store, err := pgStorage.New(ctx, logger)
	if err != nil {
		logger.Error("Ошибка инициализации БД", slog.String("", err.Error()))
	}
	defer store.Close()

	httpServer.Run(ctx, &wg, logger, store)

	wg.Wait()
	logger.Info("...done")
}
