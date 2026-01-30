module github.com/mattstam/semaphore-gnark-11

go 1.24.0

require (
	github.com/aws/aws-sdk-go v1.55.6
	github.com/consensys/gnark v0.14.0
	github.com/consensys/gnark-crypto v0.19.3-0.20251115174214-022ec58e8c19
	github.com/urfave/cli/v2 v2.25.7
	github.com/worldcoin/ptau-deserializer v0.2.0
)

require (
	github.com/bits-and-blooms/bitset v1.24.0 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/google/pprof v0.0.0-20250820193118-f64d9cf942d6 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ronanh/intcomp v1.1.1 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/consensys/gnark => github.com/p4u/gnark v0.0.0-20251217225531-cd7874155e26

replace github.com/worldcoin/ptau-deserializer => github.com/worldcoin/ptau-deserializer v0.2.1-0.20251216165311-8939a620c4a1
