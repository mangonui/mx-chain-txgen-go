// genwallet — minimal MultiversX wallet PEM generator.
//
// Why this exists: the canonical way to create a MultiversX wallet PEM
// is `mxpy wallet new --format pem --outfile wallet.pem`, but mxpy
// requires a Python install + pipx + the SDK CLI package. Since this
// repo already depends on mx-sdk-go, we can produce the same PEM
// shape with ~30 lines of Go and no extra runtime dependencies. The
// output is byte-compatible with mxpy-generated PEMs.
//
// Usage:
//
//	go run ./cmd/genwallet --out path/to/wallet.pem
//
// Or once built via `make build`:
//
//	go build -o cmd/genwallet/genwallet ./cmd/genwallet
//	./cmd/genwallet/genwallet --out examples/devnet-smoke/wallet.pem
//
// After generation, fund the printed bech32 address via the faucet on
// https://devnet-wallet.multiversx.com (devnet) or
// https://testnet-wallet.multiversx.com (testnet).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/multiversx/mx-chain-crypto-go/signing"
	"github.com/multiversx/mx-chain-crypto-go/signing/ed25519"
	sdkInteractors "github.com/multiversx/mx-sdk-go/interactors"
)

func main() {
	outPath := flag.String("out", "wallet.pem", "PEM output path")
	flag.Parse()

	suite := ed25519.NewEd25519()
	keyGen := signing.NewKeyGenerator(suite)
	sk, _ := keyGen.GeneratePair()
	skBytes, err := sk.ToByteArray()
	if err != nil {
		log.Fatalf("export private key: %v", err)
	}

	w := sdkInteractors.NewWallet()
	if err := w.SavePrivateKeyToPemFile(skBytes, *outPath); err != nil {
		log.Fatalf("save pem to %s: %v", *outPath, err)
	}

	addr, err := w.GetAddressFromPrivateKey(skBytes)
	if err != nil {
		log.Fatalf("derive address: %v", err)
	}
	bech32, err := addr.AddressAsBech32String()
	if err != nil {
		log.Fatalf("bech32 encode: %v", err)
	}

	abs, err := filepath.Abs(*outPath)
	if err != nil {
		abs = *outPath
	}
	fmt.Fprintf(os.Stdout, "address: %s\n", bech32)
	fmt.Fprintf(os.Stdout, "pem:     %s\n", abs)
	fmt.Fprintf(os.Stdout, "next:    fund this address via https://devnet-wallet.multiversx.com\n")
	fmt.Fprintf(os.Stdout, "         (or https://testnet-wallet.multiversx.com for testnet)\n")
}
