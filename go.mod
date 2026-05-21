module github.com/mangonui/mx-chain-txgen-go

go 1.23.0

toolchain go1.23.6

require (
	github.com/BurntSushi/toml v1.4.0
	github.com/gin-gonic/gin v1.10.0
	github.com/multiversx/mx-chain-core-go v1.5.0
	github.com/multiversx/mx-chain-crypto-go v1.3.1
	github.com/multiversx/mx-sdk-go v1.4.8
)

require (
	github.com/beevik/ntp v1.3.0 // indirect
	github.com/btcsuite/btcd/btcutil v1.1.3 // indirect
	github.com/bytedance/sonic v1.11.6 // indirect
	github.com/bytedance/sonic/loader v0.1.1 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/denisbrodbeck/machineid v1.0.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.6 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.20.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiversx/concurrent-map v0.1.4 // indirect
	github.com/multiversx/mx-chain-communication-go v1.3.1 // indirect
	github.com/multiversx/mx-chain-go v1.10.0 // indirect
	github.com/multiversx/mx-chain-logger-go v1.1.0 // indirect
	github.com/multiversx/mx-chain-storage-go v1.1.0 // indirect
	github.com/multiversx/mx-chain-vm-common-go v1.6.6 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pelletier/go-toml v1.9.3 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/tklauser/go-sysconf v0.3.4 // indirect
	github.com/tklauser/numcpus v0.2.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/tyler-smith/go-bip39 v1.1.0 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/arch v0.8.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/protobuf v1.36.4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/multiversx/mx-chain-core-go => github.com/mangonui/mx-chain-core-go v0.0.0-20260514035830-0e3a1d482b18

replace github.com/multiversx/mx-chain-go => github.com/mangonui/mx-chain-go v0.0.0-20260521054523-8b1901b18b47

replace github.com/multiversx/mx-chain-crypto-go => github.com/mangonui/mx-chain-crypto-go v0.0.0-20260521050229-0071ade87e85

replace github.com/multiversx/mx-chain-logger-go => github.com/mangonui/mx-chain-logger-go v0.0.0-20260514040119-0a9c9ca2e4eb

replace github.com/multiversx/mx-chain-storage-go => github.com/mangonui/mx-chain-storage-go v0.0.0-20260514040357-ab466093f27c

replace github.com/multiversx/mx-chain-vm-common-go => github.com/mangonui/mx-chain-vm-common-go v0.0.0-20260512042020-b605c960cbd0

replace github.com/multiversx/mx-chain-scenario-go => github.com/mangonui/mx-chain-scenario-go v0.0.0-20260520094333-f2be87c9c9ce

replace github.com/multiversx/mx-chain-communication-go => github.com/mangonui/mx-chain-communication-go v0.0.0-20260514041114-09dd41ef476e

replace github.com/multiversx/mx-chain-vm-go => github.com/mangonui/mx-chain-vm-go v0.0.0-20260521050533-8997548b7f24

replace github.com/multiversx/mx-chain-es-indexer-go => github.com/mangonui/mx-chain-es-indexer-go v0.0.0-20260521054111-6d312e93ce19

replace github.com/multiversx/mx-chain-vm-v1_2-go => github.com/mangonui/mx-chain-vm-v1_2-go v0.0.0-20260514040504-9320ef19765e

replace github.com/multiversx/mx-chain-vm-v1_3-go => github.com/mangonui/mx-chain-vm-v1_3-go v0.0.0-20260514040521-bd7b757b5f9b

replace github.com/multiversx/mx-chain-vm-v1_4-go => github.com/mangonui/mx-chain-vm-v1_4-go v0.0.0-20260520095016-2434270b138c

replace github.com/multiversx/mx-components-big-int => github.com/mangonui/mx-components-big-int v0.0.0-20260507133911-536f4799b94f

replace github.com/multiversx/mx-sdk-go => github.com/mangonui/mx-sdk-go v0.0.0-20260512162209-703f18632b62

replace github.com/herumi/bls-go-binary => github.com/mangonui/bls-go-binary v0.0.0-20250924002409-446538da6433

replace github.com/multiversx/mx-chain-proxy-go => github.com/mangonui/mx-chain-proxy-go v0.0.0-20260521054522-bc31dd7c9e33
