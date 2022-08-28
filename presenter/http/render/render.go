package render

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/omni/tokenbridge-monitor/logging"
)

func JSON(w http.ResponseWriter, r *http.Request, status int, res interface{}) {
	enc := json.NewEncoder(w)

	if pretty, _ := strconv.ParseBool(r.URL.Query().Get("pretty")); pretty {
		enc.SetIndent("", "  ")
	}

	w.WriteHeader(status)
	if err := enc.Encode(res); err != nil {
		Error(w, r, fmt.Errorf("failed to marshal JSON result: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	logger := logging.LoggerFromContext(r.Context())
	logger.WithError(err).Error("request handling failed")
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
