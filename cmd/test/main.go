package main

import (
	"context"
	"flag"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

const serviceName = "jocasta.pipeliner"

// @title Pipeliner API
// @version 0.1

// @host localhost:8181
// @BasePath /api/pipeliner/v1
//
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

	attach := fileregistry.AttachInfo{
		FileID: "1111",
		Name:   "test",
	}

	attach2 := entity.Attachment{
		FileID: "1111",
	}

	fileRegistryService.GetAttachmentLink(context.Background(), []fileregistry.AttachInfo{attach})

	fileRegistryService.GetAttachments(context.Background(), []entity.Attachment{attach2}, "J00000002222", "")
}
