package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mangonui/mx-chain-txgen-go/scenarios"
	"github.com/mangonui/mx-chain-txgen-go/shards"
	"github.com/mangonui/mx-chain-txgen-go/stats"
	"github.com/mangonui/mx-chain-txgen-go/version"
)

// reportWindows defines the rolling intervals exposed by GET /stats. The
// names land verbatim in the JSON response, so changing them is a
// user-visible contract change.
var reportWindows = []stats.Window{
	{Name: "1m", Duration: time.Minute},
	{Name: "5m", Duration: 5 * time.Minute},
	{Name: "1h", Duration: time.Hour},
}

// handler is the per-request adapter from the gin context to the scenario
// registry. Stateless apart from the captured components, registry, and
// optional stats sampler.
//
// requestMu serialises /transaction/send-multiple. Without it, two
// concurrent requests can interleave Nonces.Next calls and submit
// transactions out of nonce order — the chain then rejects the lower-
// nonce tx as nonceTooLow, silently dropping load. The upstream txgen
// is also driven sequentially (one curl every N seconds in the shell
// drivers), so serialising here matches operational reality without
// surprising anyone. Diagnostic and stats endpoints are not gated.
type handler struct {
	registry  *scenarios.Registry
	comp      *scenarios.Components
	sampler   *stats.Sampler
	requestMu sync.Mutex
}

func (h *handler) sendMultiple(c *gin.Context) {
	h.requestMu.Lock()
	defer h.requestMu.Unlock()

	var req SendMultipleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "decode_failed", err.Error())
		return
	}
	if req.NumOfTxs < 0 {
		h.respondError(c, http.StatusBadRequest, "invalid_numOfTxs", "numOfTxs must be >= 0")
		return
	}
	if req.GasPrice == 0 {
		h.respondError(c, http.StatusBadRequest, "invalid_gasPrice", "gasPrice must be > 0")
		return
	}
	if req.GasLimit == 0 {
		h.respondError(c, http.StatusBadRequest, "invalid_gasLimit", "gasLimit must be > 0")
		return
	}
	dest, err := shards.ParseDestination(req.Destination)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, "invalid_destination", err.Error())
		return
	}
	scen, err := h.registry.Lookup(req.Scenario)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, "unknown_scenario", err.Error())
		return
	}

	// Normalise Version: callers omitting the field land here at 0;
	// the chain rejects Version=0 so we default to 1 (the historical
	// move-balance / contract-call shape). Operators wanting hash-on-
	// sign or guarded/relayed semantics set Version=2 explicitly.
	version := req.Version
	if version == 0 {
		version = 1
	}
	scenReq := scenarios.Request{
		Value:       req.Value.String(),
		NumOfTxs:    req.NumOfTxs,
		GasPrice:    req.GasPrice,
		GasLimit:    req.GasLimit,
		Destination: dest,
		RecallNonce: req.RecallNonce,
		Data:        req.Data,
		SCAddress:   req.SCAddress,
		Version:     version,
		Options:     req.Options,
	}
	result, err := scen.Run(c.Request.Context(), scenReq, h.comp)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "scenario_failed", err.Error())
		return
	}

	if h.sampler != nil {
		h.sampler.Record(req.Scenario, result.NumSent)
	}

	hashesMap := make(map[int]string, len(result.Hashes))
	for i, h := range result.Hashes {
		hashesMap[i] = h
	}
	c.JSON(http.StatusOK, SendMultipleResponse{
		Data: SendMultipleResponseData{
			NumOfSentTxs: result.NumSent,
			TxsHashes:    hashesMap,
			Scenario:     req.Scenario,
			SubCommand:   req.Data,
			Extra:        result.Extra,
		},
		Code: "successful",
	})
}

func (h *handler) status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"scenarios": h.registry.Names(),
			"poolSize":  h.comp.Pool.Len(),
			"build": gin.H{
				"version":   version.Version,
				"commit":    version.Commit,
				"buildDate": version.BuildDate,
			},
		},
		"code": "successful",
	})
}

// healthz is a zero-dependency liveness probe. Returns 200 OK as long as
// the process can serve HTTP. Deliberately does not check the proxy,
// the pool, or the registry — a liveness probe that turns red on a
// transient downstream failure produces restart-spiral noise without
// improving observability. Use /status for readiness-style checks.
func (h *handler) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// stats returns the rolled-up submitted-TPS report. When the sampler is
// nil (txgen launched with Stats.EnableTPSSampler=false), the windows
// array is returned empty so clients can still distinguish "disabled"
// from "no traffic".
func (h *handler) stats(c *gin.Context) {
	if h.sampler == nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"enabled": false,
				"windows": []any{},
			},
			"code": "successful",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"enabled": true,
			"windows": h.sampler.Report(reportWindows),
		},
		"code": "successful",
	})
}

func (h *handler) respondError(c *gin.Context, status int, code, msg string) {
	c.JSON(status, SendMultipleResponse{
		Error: msg,
		Code:  code,
	})
}
