package humantasks

import (
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	d "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
)

type ServiceWithCache struct {
	Cache      cachekit.Cache
	Humantasks HumantasksInterface
}

type Service struct {
	C     *grpc.ClientConn
	Cli   d.DelegationServiceClient
	Cache cachekit.Cache
}

func NewServiceWithCache(cfg Config) (HumantasksInterface, error) {
	service, err := NewService(cfg)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.CacheConfig))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &ServiceWithCache{
		Humantasks: service,
		Cache:      cache,
	}, nil
}

func NewService(cfg Config) (HumantasksInterface, error) {
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
