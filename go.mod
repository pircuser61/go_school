module gitlab.services.mts.ru/jocasta/pipeliner

go 1.16

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	github.com/deepmap/oapi-codegen v1.11.0
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/chi/v5 v5.0.7
	github.com/go-chi/render v1.0.1
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/uuid v1.3.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.1-0.20190118093823-f849b5445de4
	github.com/jackc/pgx/v4 v4.15.0
	github.com/lib/pq v1.10.4 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pressly/goose v2.7.0+incompatible
	github.com/prometheus/client_golang v1.10.0
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/stretchr/testify v1.7.1
	github.com/swaggo/http-swagger v1.0.0
	github.com/swaggo/swag v1.7.0
	gitlab.services.mts.ru/abp/mail/pkg v1.18.6
	gitlab.services.mts.ru/abp/myosotis v1.4.1
	gitlab.services.mts.ru/erius/monitoring v0.1.0
	gitlab.services.mts.ru/erius/network-monitor-client v1.0.5
	gitlab.services.mts.ru/erius/scheduler_client v0.0.5
	gitlab.services.mts.ru/libs/logger v1.1.1
	go.opencensus.io v0.23.0
	google.golang.org/genproto v0.0.0-20210903162649-d08c68adba83 // indirect
	google.golang.org/grpc v1.40.0
	gopkg.in/square/go-jose.v2 v2.6.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)
