package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	sdkData "github.com/multiversx/mx-sdk-go/data"

	"github.com/mangonui/mx-chain-txgen-go/accounts"
	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/scenarios"
	"github.com/mangonui/mx-chain-txgen-go/shards"
	"github.com/mangonui/mx-chain-txgen-go/stats"
)

// fakeScenario is a Scenario implementation that produces deterministic
// canned results so the HTTP layer can be exercised end-to-end without a
// live proxy or chain.
type fakeScenario struct {
	name       string
	calls      int32
	returnErr  error
	hashesFunc func(req scenarios.Request) []string
	extra      map[string]any
}

func (f *fakeScenario) Name() string { return f.name }

func (f *fakeScenario) Run(_ context.Context, req scenarios.Request, _ *scenarios.Components) (*scenarios.Result, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	hashes := []string{}
	if f.hashesFunc != nil {
		hashes = f.hashesFunc(req)
	}
	return &scenarios.Result{
		NumSent: len(hashes),
		Hashes:  hashes,
		Extra:   f.extra,
	}, nil
}

// newTestEngine wires a minimal handler against a fake scenario registry
// and (optionally) a stats sampler. Returns the gin engine so callers
// drive it through httptest.NewRecorder, plus the registered fake so they
// can assert call count.
func newTestEngine(t *testing.T, fake *fakeScenario, sampler *stats.Sampler) (*gin.Engine, *fakeScenario) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(loggingMiddleware())

	registry, err := scenarios.NewRegistry(
		[]scenarios.Scenario{fake},
		[]string{fake.name},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}

	// Build a minimal Components — only Pool is read by the /status
	// handler, the rest is captured by reference but never dereferenced
	// in the handler-layer tests.
	sc, err := shards.NewCoordinator(2)
	if err != nil {
		t.Fatalf("shard coordinator: %v", err)
	}
	cfg := config.AccountsConfig{
		PoolSize:          2,
		StateDir:          t.TempDir(),
		RegenerateOnStart: true,
		InitialBalance:    "1",
	}
	pool, err := accounts.NewPool(cfg, sc)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	comp := &scenarios.Components{
		Pool:      pool,
		Shards:    sc,
		NetConfig: &sdkData.NetworkConfig{ChainID: "test", MinGasPrice: 1_000_000_000},
		Cfg:       &config.Config{},
	}

	h := &handler{registry: registry, comp: comp, sampler: sampler}
	engine.POST("/transaction/send-multiple", h.sendMultiple)
	engine.GET("/status", h.status)
	engine.GET("/stats", h.stats)
	engine.GET("/healthz", h.healthz)
	return engine, fake
}

// doPost serialises body to JSON and drives the engine through an
// httptest recorder. Returns the recorder so callers inspect status +
// body.
func doPost(t *testing.T, engine *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(buf)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestHandler_SuccessPath(t *testing.T) {
	fake := &fakeScenario{
		name: "basic",
		hashesFunc: func(req scenarios.Request) []string {
			out := make([]string, req.NumOfTxs)
			for i := range out {
				out[i] = fmt.Sprintf("tx-%d", i)
			}
			return out
		},
	}
	engine, _ := newTestEngine(t, fake, nil)

	rec := doPost(t, engine, "/transaction/send-multiple", SendMultipleRequest{
		Value:       FlexibleAmount("1"),
		NumOfTxs:    3,
		GasPrice:    1_000_000_000,
		GasLimit:    50_000,
		Destination: "mixed",
		Scenario:    "basic",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp SendMultipleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "successful" {
		t.Fatalf("code: got %q, want successful", resp.Code)
	}
	if resp.Data.NumOfSentTxs != 3 {
		t.Fatalf("numOfSentTxs: got %d, want 3", resp.Data.NumOfSentTxs)
	}
	if resp.Data.Scenario != "basic" {
		t.Fatalf("scenario: got %q, want basic", resp.Data.Scenario)
	}
	if len(resp.Data.TxsHashes) != 3 {
		t.Fatalf("txsHashes: got %d entries, want 3", len(resp.Data.TxsHashes))
	}
	if got := atomic.LoadInt32(&fake.calls); got != 1 {
		t.Fatalf("fake scenario calls: got %d, want 1", got)
	}
}

func TestHandler_UnknownScenarioReturns400(t *testing.T) {
	fake := &fakeScenario{name: "basic"}
	engine, _ := newTestEngine(t, fake, nil)

	rec := doPost(t, engine, "/transaction/send-multiple", SendMultipleRequest{
		Value:       FlexibleAmount("1"),
		NumOfTxs:    1,
		GasPrice:    1_000_000_000,
		GasLimit:    50_000,
		Destination: "mixed",
		Scenario:    "rwa", // not registered
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	var resp SendMultipleResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != "unknown_scenario" {
		t.Fatalf("code: got %q, want unknown_scenario", resp.Code)
	}
	if !strings.Contains(resp.Error, "rwa") {
		t.Fatalf("error should mention the unknown name: %q", resp.Error)
	}
}

func TestHandler_BadJSONReturns400(t *testing.T) {
	fake := &fakeScenario{name: "basic"}
	engine, _ := newTestEngine(t, fake, nil)

	req := httptest.NewRequest(http.MethodPost, "/transaction/send-multiple",
		strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	var resp SendMultipleResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != "decode_failed" {
		t.Fatalf("code: got %q, want decode_failed", resp.Code)
	}
}

func TestHandler_MissingGasPriceReturns400(t *testing.T) {
	fake := &fakeScenario{name: "basic"}
	engine, _ := newTestEngine(t, fake, nil)

	rec := doPost(t, engine, "/transaction/send-multiple", SendMultipleRequest{
		Value:       FlexibleAmount("1"),
		NumOfTxs:    1,
		GasPrice:    0, // invalid
		GasLimit:    50_000,
		Destination: "mixed",
		Scenario:    "basic",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	var resp SendMultipleResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != "invalid_gasPrice" {
		t.Fatalf("code: got %q, want invalid_gasPrice", resp.Code)
	}
}

func TestHandler_BadDestinationReturns400(t *testing.T) {
	fake := &fakeScenario{name: "basic"}
	engine, _ := newTestEngine(t, fake, nil)

	rec := doPost(t, engine, "/transaction/send-multiple", SendMultipleRequest{
		Value:       FlexibleAmount("1"),
		NumOfTxs:    1,
		GasPrice:    1_000_000_000,
		GasLimit:    50_000,
		Destination: "moon", // invalid
		Scenario:    "basic",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	var resp SendMultipleResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != "invalid_destination" {
		t.Fatalf("code: got %q, want invalid_destination", resp.Code)
	}
}

func TestHandler_ScenarioErrorReturns500(t *testing.T) {
	fake := &fakeScenario{
		name:      "basic",
		returnErr: errors.New("simulated failure"),
	}
	engine, _ := newTestEngine(t, fake, nil)

	rec := doPost(t, engine, "/transaction/send-multiple", SendMultipleRequest{
		Value:       FlexibleAmount("1"),
		NumOfTxs:    1,
		GasPrice:    1_000_000_000,
		GasLimit:    50_000,
		Destination: "mixed",
		Scenario:    "basic",
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
	var resp SendMultipleResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Code != "scenario_failed" {
		t.Fatalf("code: got %q, want scenario_failed", resp.Code)
	}
	if !strings.Contains(resp.Error, "simulated failure") {
		t.Fatalf("error should surface the cause: %q", resp.Error)
	}
}

func TestHandler_StatsRecordsSuccessfulSubmissions(t *testing.T) {
	fake := &fakeScenario{
		name: "basic",
		hashesFunc: func(req scenarios.Request) []string {
			out := make([]string, req.NumOfTxs)
			for i := range out {
				out[i] = fmt.Sprintf("tx-%d", i)
			}
			return out
		},
	}
	sampler := stats.New(time.Hour)
	engine, _ := newTestEngine(t, fake, sampler)

	for i := 0; i < 3; i++ {
		rec := doPost(t, engine, "/transaction/send-multiple", SendMultipleRequest{
			Value:       FlexibleAmount("1"),
			NumOfTxs:    50,
			GasPrice:    1_000_000_000,
			GasLimit:    50_000,
			Destination: "mixed",
			Scenario:    "basic",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("iteration %d: status %d, body=%s", i, rec.Code, rec.Body.String())
		}
	}

	// /stats over a window long enough to cover all three submissions.
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stats", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/stats: got %d", rec.Code)
	}
	var resp struct {
		Data struct {
			Enabled bool           `json:"enabled"`
			Windows []stats.Report `json:"windows"`
		} `json:"data"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode /stats: %v body=%s", err, rec.Body.String())
	}
	if !resp.Data.Enabled {
		t.Fatalf("/stats: enabled=false, want true")
	}
	// First window is 1m — should contain all 150 txs.
	if resp.Data.Windows[0].TotalTxs != 150 {
		t.Fatalf("1m total: got %d, want 150 (windows=%+v)",
			resp.Data.Windows[0].TotalTxs, resp.Data.Windows)
	}
	if resp.Data.Windows[0].PerScenarioTxs["basic"] != 150 {
		t.Fatalf("1m basic: got %d, want 150",
			resp.Data.Windows[0].PerScenarioTxs["basic"])
	}
}

func TestHandler_StatsDisabledWhenSamplerNil(t *testing.T) {
	fake := &fakeScenario{name: "basic"}
	engine, _ := newTestEngine(t, fake, nil)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stats", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/stats: got %d", rec.Code)
	}
	var resp struct {
		Data struct {
			Enabled bool          `json:"enabled"`
			Windows []interface{} `json:"windows"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Enabled {
		t.Fatalf("enabled: got true, want false")
	}
	if len(resp.Data.Windows) != 0 {
		t.Fatalf("windows: got %d entries, want 0", len(resp.Data.Windows))
	}
}

func TestHandler_StatusEndpoint(t *testing.T) {
	fake := &fakeScenario{name: "basic"}
	engine, _ := newTestEngine(t, fake, nil)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/status: got %d", rec.Code)
	}
	var resp struct {
		Data struct {
			Scenarios []string `json:"scenarios"`
			PoolSize  int      `json:"poolSize"`
		} `json:"data"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != "successful" {
		t.Fatalf("code: got %q", resp.Code)
	}
	if len(resp.Data.Scenarios) != 1 || resp.Data.Scenarios[0] != "basic" {
		t.Fatalf("scenarios: got %+v", resp.Data.Scenarios)
	}
	if resp.Data.PoolSize != 2 {
		t.Fatalf("poolSize: got %d, want 2", resp.Data.PoolSize)
	}
}

func TestHandler_HealthzReturnsOK(t *testing.T) {
	fake := &fakeScenario{name: "basic"}
	engine, _ := newTestEngine(t, fake, nil)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz: got %d, want 200", rec.Code)
	}
	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("/healthz status: got %q, want ok", resp.Status)
	}
}

// newTestEngine registers /healthz alongside the other routes so the
// healthz test works against the same wiring the real server uses.

func TestHandler_StatusIncludesBuildInfo(t *testing.T) {
	fake := &fakeScenario{name: "basic"}
	engine, _ := newTestEngine(t, fake, nil)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/status: got %d", rec.Code)
	}
	var resp struct {
		Data struct {
			Build struct {
				Version   string `json:"version"`
				Commit    string `json:"commit"`
				BuildDate string `json:"buildDate"`
			} `json:"build"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Defaults in version/version.go are "dev" / "unknown" / "unknown".
	// Tests build via plain `go test` so ldflags aren't injected, hence
	// the defaults survive — assertions match the unflagged baseline.
	if resp.Data.Build.Version == "" {
		t.Fatalf("Build.Version is empty; want non-empty default")
	}
	if resp.Data.Build.Commit == "" {
		t.Fatalf("Build.Commit is empty; want non-empty default")
	}
	if resp.Data.Build.BuildDate == "" {
		t.Fatalf("Build.BuildDate is empty; want non-empty default")
	}
}

func TestHandler_SubmittedHashesArePreservedInResponse(t *testing.T) {
	expected := []string{"hash-A", "hash-B", "hash-C"}
	fake := &fakeScenario{
		name:       "basic",
		hashesFunc: func(req scenarios.Request) []string { return expected },
	}
	engine, _ := newTestEngine(t, fake, nil)

	rec := doPost(t, engine, "/transaction/send-multiple", SendMultipleRequest{
		Value:       FlexibleAmount("1"),
		NumOfTxs:    3,
		GasPrice:    1_000_000_000,
		GasLimit:    50_000,
		Destination: "mixed",
		Scenario:    "basic",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var resp SendMultipleResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	for i, want := range expected {
		if got := resp.Data.TxsHashes[i]; got != want {
			t.Fatalf("hash %d: got %q, want %q", i, got, want)
		}
	}
}
