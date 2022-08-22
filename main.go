package main

import (
	"encoding/json"
	"flag"
	"fmt"
	errorLog "log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/alertmanager/template"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type httpHandler struct {
	Logger *zap.Logger
}

func newZapConfig() zap.Config {
	config := zap.NewProductionConfig()

	// Use more common logging fields than the default ones
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.MessageKey = "message"

	// Log normal logs to stdout, error logs to stderr
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}
	return config
}

func main() {
	address := flag.String("address", ":8000", "address and port of service")
	flag.Parse()

	zapConfig := newZapConfig()
	logger, err := zapConfig.Build()
	if err != nil {
		panic(err)
	}

	h := httpHandler{
		Logger: logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/alerts", h.alertHandler).Methods("POST")
	router.HandleFunc("/ready", h.readyHandler).Methods("GET")

	srv := &http.Server{
		Handler: router,
		Addr:    *address,

		WriteTimeout: 5 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	logger.Debug("Starting webhook receiver")
	errorLog.Fatal(srv.ListenAndServe())
}

func (h *httpHandler) readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *httpHandler) alertHandler(w http.ResponseWriter, r *http.Request) {
	var alerts template.Data

	// Check the payload is actually an Alertmanager payload.
	err := json.NewDecoder(r.Body).Decode(&alerts)
	if err != nil {
		h.Logger.Error(fmt.Errorf("cannot parse request: %w", err).Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	output, err := json.Marshal(alerts)
	if err != nil {
		h.Logger.Error(fmt.Errorf("cannot serialize back alerts: %w", err).Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logger := h.Logger.With(
		zap.Any("alerts", json.RawMessage(output)),
	)

	var log func(msg string, fields ...zapcore.Field)

	if alerts.Status == "firing" {
		log = logger.Warn
	} else if alerts.Status == "resolved" {
		log = logger.Info
	} else {
		log = logger.Error
	}

	log("Events received")
	w.WriteHeader(http.StatusOK)
}
