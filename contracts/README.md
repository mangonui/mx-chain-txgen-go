# contracts/

This directory holds the compiled wasm contract used by the `erc20`
scenario.

The repo intentionally does **not** bundle a pre-compiled wasm. Operators
supply their own so the scenario can be exercised against the contract
flavour they actually care about.

## Expected wasm ABI

The `erc20` scenario invokes the contract through three endpoints:

| Op | Caller | Data field |
|---|---|---|
| Deploy | pool index 0 (`erc20DeployerIndex`) | `<wasm_hex>@0500@0500@<initialSupply_hex>` |
| Mint | deployer | `mint@<receiver_pubkey_hex>@<amount_hex>` |
| Transfer | any holder | `transfer@<receiver_pubkey_hex>@<amount_hex>` |

The wasm must therefore expose:

- A constructor (`init`) that accepts `(initial_supply: BigUint)` and
  credits it to the caller.
- An owner-only `mint(receiver: ManagedAddress, amount: BigUint)` endpoint
  that credits `amount` to `receiver`.
- A `transfer(receiver: ManagedAddress, amount: BigUint)` endpoint that
  moves `amount` from caller's balance to `receiver`'s balance.

## Where to get one

The canonical reference is the ERC20 example in
[`multiversx/mx-sdk-rs`](https://github.com/multiversx/mx-sdk-rs/tree/master/contracts/examples/erc20).
That contract's `init`/`mint`/`transfer` signatures match the ABI above.

Build instructions (from the mx-sdk-rs root):

```bash
cd contracts/examples/erc20
sc-meta all build
cp output/erc20.wasm <this-repo>/contracts/erc20.wasm
```

## Path resolution

`config.toml` defaults the path to `./contracts/erc20.wasm` (relative to
the working directory when launching `cmd/txgen/txgen`). Override with
`ERC20.WasmPath` if you keep the wasm elsewhere.

## Why no fallback

A fallback (procedurally-generated stub wasm at startup) would silently
mask "operator forgot to provide a real ERC20" mistakes and produce
load-test results that don't reflect any real contract's behaviour. The
scenario instead fails loudly at deploy time when the file is missing
or invalid.
