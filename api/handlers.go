package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mangonui/mx-chain-txgen-go/scenarios"
	"github.com/mangonui/mx-chain-txgen-go/shards"
)

// handler is the per-request adapter from the gin context to the scenario
// registry. Stateless apart from the captured components and registry.
type handler struct {
	registry *scenarios.Registry
	comp     *scenarios.Components
}

func (h *handler) sendMultiple(c *gin.Context) {
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

	scenReq := scenarios.Request{
		Value:       req.Value.String(),
		NumOfTxs:    req.NumOfTxs,
		GasPrice:    req.GasPrice,
		GasLimit:    req.GasLimit,
		Destination: dest,
		RecallNonce: req.RecallNonce,
		Data:        req.Data,
		SCAddress:   req.SCAddress,
	}
	result, err := scen.Run(c.Request.Context(), scenReq, h.comp)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "scenario_failed", err.Error())
		return
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
