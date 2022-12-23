module gitlab.services.mts.ru/jocasta/pipeliner

go 1.18

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	github.com/Shopify/sarama v1.37.2
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/deepmap/oapi-codegen v1.11.0
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/chi/v5 v5.0.7
	github.com/go-chi/render v1.0.1
	github.com/google/uuid v1.3.0
	github.com/iancoleman/orderedmap v0.2.0
	github.com/jackc/pgconn v1.13.0
	github.com/jackc/pgx/v4 v4.17.2
	github.com/lib/pq v1.10.7
	github.com/pkg/errors v0.9.1
	github.com/pressly/goose v2.7.0+incompatible
	github.com/prometheus/client_golang v1.13.0
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/stretchr/testify v1.8.1
	github.com/swaggo/swag v1.7.0
	gitlab.services.mts.ru/abp/mail/pkg v1.18.7
	gitlab.services.mts.ru/abp/myosotis v1.4.4
	gitlab.services.mts.ru/erius/monitoring v0.1.0
	gitlab.services.mts.ru/erius/network-monitor-client v1.0.5
	gitlab.services.mts.ru/erius/scheduler_client v0.0.5
	gitlab.services.mts.ru/jocasta/functions v1.3.0-alpha.3
	gitlab.services.mts.ru/jocasta/msg-kit v0.1.1
	go.opencensus.io v0.24.0
	golang.org/x/net v0.2.0
	google.golang.org/grpc v1.51.0
	gopkg.in/square/go-jose.v2 v2.6.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.0 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/eapache/go-resiliency v1.3.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/getsentry/sentry-go v0.11.0 // indirect
	github.com/go-kit/log v0.2.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/spec v0.20.0 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.14.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.1 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgtype v1.12.0 // indirect
	github.com/jackc/puddle v1.3.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.3 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/makasim/sentryhook v0.4.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/pierrec/lz4/v4 v4.1.17 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/streadway/amqp v1.0.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/uber/jaeger-client-go v2.25.0+incompatible // indirect
	gitlab.services.mts.ru/jocasta/human-tasks v1.0.0-alpha.3 // indirect
	golang.org/x/crypto v0.0.0-20220817201139-bc19a97f63c8 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.2.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	golang.org/x/tools v0.1.12 // indirect
	google.golang.org/api v0.81.0 // indirect
	google.golang.org/genproto v0.0.0-20221205194025-8222ab48f5fc // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
