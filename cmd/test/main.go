package main

import (
	"context"
	"flag"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

//nolint:gocyclo //it's ok here
func main() {
	configPath := flag.String("c", "cmd/pipeliner/config.yaml", "path to config")
	flag.Parse()

	log := logger.CreateLogger(nil)

	cfg := &configs.Pipeliner{}

	err := configs.Read(*configPath, cfg)
	if err != nil {
		log.WithError(err).Fatal("can't read config")
	}

	log = logger.CreateLogger(cfg.Log)

	metrics.InitMetricsAuth(cfg.Prometheus)

	m := metrics.New(cfg.Prometheus)

	fileRegistryService, err := file_registry.NewService(cfg.FileRegistry, log, m)
	if err != nil {
		log.WithError(err).Error("can't create file-registry service")

		return
	}

	attach := file_registry.AttachInfo{
		FileID: "1111",
		Name:   "test",
	}

	attach2 := entity.Attachment{
		FileID: "1111",
	}

	_, _ = fileRegistryService.GetAttachmentLink(context.Background(), []file_registry.AttachInfo{attach})

	_, _ = fileRegistryService.GetAttachments(context.Background(), []entity.Attachment{attach2}, "J00000002222", "")
}
