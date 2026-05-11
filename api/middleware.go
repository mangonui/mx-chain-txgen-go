package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// requestIDCounter is a monotonic counter used in lieu of UUIDs. The
// request_id is for log correlation, not security; an in-process
// counter is sufficient and faster than UUID generation.
var requestIDCounter uint64

// requestObservation is the structured form of a single request log line.
type requestObservation struct {
	RequestID  uint64  `json:"req"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Status     int     `json:"status"`
	DurationMS float64 `json:"durationMs"`

	// Scenario / sub-command / numOfTxs are only populated for
	// /transaction/send-multiple. They are sniffed out of the request
	// body without re-parsing the full JSON each time the route runs —
	// the body is read into a buffer, parsed once for fields we care
	// about, and the buffer is replaced on the context so the actual
	// handler still sees the original body.
	Scenario   string `json:"scenario,omitempty"`
	SubCommand string `json:"subCommand,omitempty"`
	NumOfTxs   int    `json:"numOfTxs,omitempty"`
}

// loggingMiddleware emits one structured log line per request. The
// scenario/sub-command/numOfTxs fields are extracted from the JSON body
// of /transaction/send-multiple before the handler runs. Read errors
// are swallowed (logging is best-effort and must not break the request).
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		obs := requestObservation{
			RequestID: atomic.AddUint64(&requestIDCounter, 1),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
		}

		if c.Request.Method == "POST" && c.Request.URL.Path == "/transaction/send-multiple" && c.Request.Body != nil {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewReader(body))
				var peek struct {
					Scenario string `json:"scenario"`
					Data     string `json:"data"`
					NumOfTxs int    `json:"numOfTxs"`
				}
				if jerr := json.Unmarshal(body, &peek); jerr == nil {
					obs.Scenario = peek.Scenario
					obs.SubCommand = peek.Data
					obs.NumOfTxs = peek.NumOfTxs
				}
			}
		}

		c.Set("request_id", obs.RequestID)
		start := time.Now()
		c.Next()
		obs.DurationMS = float64(time.Since(start).Microseconds()) / 1000.0
		obs.Status = c.Writer.Status()

		log.Printf("req=%d %s %s status=%d duration=%sms%s",
			obs.RequestID, obs.Method, obs.Path, obs.Status,
			strconv.FormatFloat(obs.DurationMS, 'f', 3, 64),
			scenarioSuffix(obs))
	}
}

// scenarioSuffix appends the scenario/sub-command/numOfTxs trio only
// when at least one is populated. Keeps log lines for /status,
// /healthz, /stats compact.
func scenarioSuffix(obs requestObservation) string {
	if obs.Scenario == "" && obs.SubCommand == "" && obs.NumOfTxs == 0 {
		return ""
	}
	return fmt.Sprintf(" scenario=%s sub=%s numOfTxs=%d",
		obs.Scenario, obs.SubCommand, obs.NumOfTxs)
}
