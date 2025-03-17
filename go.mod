module github.com/prometheus/common

go 1.23.0

toolchain go1.24.1

require (
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/google/go-cmp v0.7.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f
	github.com/prometheus/client_model v0.6.1
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/pdata v1.27.0
	golang.org/x/net v0.35.0
	golang.org/x/oauth2 v0.25.0
	google.golang.org/protobuf v1.36.5
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.20.4 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241202173237-19429a94021a // indirect
	google.golang.org/grpc v1.70.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract v0.50.0 // Critical bug in counter suffixes, please read issue https://github.com/prometheus/common/issues/605
