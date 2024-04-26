module gitlab.services.mts.ru/jocasta/pipeliner

go 1.18

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	github.com/Shopify/sarama v1.37.2
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de
	github.com/deepmap/oapi-codegen v1.12.4
	github.com/emersion/go-imap v1.2.1
	github.com/emersion/go-message v0.15.0
	github.com/fatih/structs v1.1.0
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/chi/v5 v5.0.8
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.1.0
	github.com/hrishin/httpmock v0.0.2
	github.com/iancoleman/orderedmap v0.3.0
	github.com/jackc/pgconn v1.14.0
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa
	github.com/jackc/pgx/v4 v4.18.1
	github.com/lib/pq v1.10.9
	github.com/pkg/errors v0.9.1
	github.com/pressly/goose/v3 v3.7.0
	github.com/prometheus/client_golang v1.14.0
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/stretchr/testify v1.9.0
	github.com/xeipuuv/gojsonschema v1.2.0
	gitlab.services.mts.ru/abp/mail/pkg v1.19.0
	gitlab.services.mts.ru/abp/myosotis v1.4.4
	gitlab.services.mts.ru/jocasta/cache-kit v1.2.0
	gitlab.services.mts.ru/jocasta/conditions-kit v1.0.10
	gitlab.services.mts.ru/jocasta/file-registry v1.4.0
	gitlab.services.mts.ru/jocasta/forms v1.7.0-rc.1.0.20240301073647-dad697e5b82d
	gitlab.services.mts.ru/jocasta/functions v1.11.0-alpha.1
	gitlab.services.mts.ru/jocasta/human-tasks v1.0.0-alpha.3
	gitlab.services.mts.ru/jocasta/integrations v1.9.0
	gitlab.services.mts.ru/jocasta/msg-kit v0.1.4
	gitlab.services.mts.ru/jocasta/scheduler v1.10.1-0.20240411105447-0c6a180ab838
	gitlab.services.mts.ru/jocasta/sequence v0.0.0-20240412094937-d4b3accaf9cd
	gitlab.services.mts.ru/prodboard/infra v0.3.0
	go.opencensus.io v0.24.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
	golang.org/x/net v0.21.0
	golang.org/x/sync v0.6.0
	golang.org/x/text v0.14.0
	google.golang.org/grpc v1.61.1
	gopkg.in/square/go-jose.v2 v2.6.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/eapache/go-resiliency v1.3.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/emersion/go-sasl v0.0.0-20200509203442-7bfe0ed36a21 // indirect
	github.com/emersion/go-textwrapper v0.0.0-20200911093747-65d896831594 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/getsentry/sentry-go v0.20.0 // indirect
	github.com/go-kit/log v0.2.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-redis/redis v6.15.9+incompatible // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.2 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgtype v1.14.0 // indirect
	github.com/jackc/puddle v1.3.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.3 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/makasim/sentryhook v0.4.0 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.17 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/streadway/amqp v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/uber/jaeger-client-go v2.25.0+incompatible // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	golang.org/x/crypto v0.19.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	google.golang.org/api v0.114.0 // indirect
	google.golang.org/genproto v0.0.0-20240213162025-012b6fc9bca9 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/protobuf v1.32.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
