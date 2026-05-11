# Wiring mx-chain-txgen-go (mangonui fork) into the mx-chain-go testnet scripts

`mx-chain-go/scripts/testnet/prerequisites.sh` line 62 of the v2.0.0
snapshot clones the upstream private repo via SSH:

```bash
git clone git@github.com:multiversx/mx-chain-txgen-go.git
```

That clone fails for anyone without access to the upstream private repo
(the URL returns HTTP 404 publicly). To wire this fork in, apply this
one-line patch on top of the v2.0.0 scripts:

```diff
--- a/scripts/testnet/prerequisites.sh
+++ b/scripts/testnet/prerequisites.sh
@@ -59,7 +59,7 @@ if [ $USE_TXGEN -eq 1 ]; then
   if [ ! -d $TXGENDIR ]
   then
     pushd $(dirname $MULTIVERSXDIR)
-    git clone git@github.com:multiversx/mx-chain-txgen-go.git
+    git clone git@github.com:mangonui/mx-chain-txgen-go.git
     popd
   fi
 fi
```

After applying:

1. Set `USE_TXGEN=1` in `scripts/testnet/variables.sh`.
2. Run `./prerequisites.sh && ./config.sh && ./start.sh` from
   `mx-chain-go/scripts/testnet/`.

`config.sh` already copies `node/config/walletKey.pem` into the txgen
config directory (line 239). With `[Faucet]` enabled in this repo's
`config.toml`, the txgen will pre-fund every pool account from that
walletKey before serving any scenario requests.

## Alternative: do not patch the upstream scripts

If you prefer not to touch `mx-chain-go/scripts/testnet/`, clone the
fork manually as a sibling of `mx-chain-go` before running
`start.sh`:

```bash
cd $(dirname $(pwd))   # one level above mx-chain-go
git clone git@github.com:mangonui/mx-chain-txgen-go.git
```

`prerequisites.sh` will see the directory already exists and skip the
clone. `config.sh` will still copy the walletKey and edit the
scenarios line. `start.sh` will build and launch the txgen binary at
`./cmd/txgen/txgen` and the upstream `tools.sh runTxGen` wrapper will
launch it with `-num-accounts $NUMACCOUNTS -new-accounts` on first run.
