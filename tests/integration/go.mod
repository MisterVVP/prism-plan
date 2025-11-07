module prismtest

go 1.24.0

toolchain go1.24.3

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.19.1
	github.com/Azure/azure-sdk-for-go/sdk/data/aztables v1.4.0
	github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue v1.0.1
	github.com/redis/go-redis/v9 v9.14.1
	prismtestutil v0.0.0
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/text v0.30.0 // indirect
)

replace prismtestutil => ../utils
