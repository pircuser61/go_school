module gitlab.services.mts.ru/erius/pipeliner

go 1.13

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.0
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/cors v1.1.1
	github.com/go-openapi/jsonreference v0.19.4 // indirect
	github.com/google/uuid v1.1.1
	github.com/jackc/pgx/v4 v4.6.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.0
	github.com/swaggo/http-swagger v0.0.0-20200308142732-58ac5e232fba
	github.com/swaggo/swag v1.6.7
	gitlab.services.mts.ru/erius/admin v0.0.24
	gitlab.services.mts.ru/libs/logger v1.1.1
	go.opencensus.io v0.22.3
	golang.org/x/tools v0.0.0-20200721032237-77f530d86f9a // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
)
