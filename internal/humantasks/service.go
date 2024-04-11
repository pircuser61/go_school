package humantasks

import (
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	d "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
)

func NewServiceWithCache(cfg *Config) (ServiceInterface, error) {
	service, err := NewService(cfg)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.CacheConfig))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &ServiceWithCache{
		Cache:      cache,
		Humantasks: service,
	}, nil
}

func NewService(cfg *Config) (ServiceInterface, error) {
	if cfg.URL == "" {
		return &ServiceWithCache{}, nil
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
	}

	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	client := d.NewDelegationServiceClient(conn)

	return &Service{
		C:   conn,
		Cli: client,
	}, nil
}
