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

	handler := httpHandler{
		Logger: logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/alerts", handler.alertHandler).Methods("POST")
	router.HandleFunc("/ready", handler.readyHandler).Methods("GET")

	var (
		writeTimeout = 5 * time.Second
		readTimeout  = 15 * time.Second
	)

	srv := &http.Server{
		Handler: router,
		Addr:    *address,

		WriteTimeout: writeTimeout,
		ReadTimeout:  readTimeout,
	}

	logger.Debug("Starting webhook receiver")
	errorLog.Fatal(srv.ListenAndServe())
}

func (h *httpHandler) readyHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *httpHandler) alertHandler(writer http.ResponseWriter, r *http.Request) {
	var alerts template.Data

	// Check the payload is actually an Alertmanager payload.
	err := json.NewDecoder(r.Body).Decode(&alerts)
	if err != nil {
		h.Logger.Error(fmt.Errorf("cannot parse request: %w", err).Error())
		http.Error(writer, err.Error(), http.StatusBadRequest)

		return
	}

	output, err := json.Marshal(alerts)
	if err != nil {
		h.Logger.Error(fmt.Errorf("cannot serialize back alerts: %w", err).Error())
		http.Error(writer, err.Error(), http.StatusBadRequest)

		return
	}

	logger := h.Logger.With(
		zap.Any("alerts", json.RawMessage(output)),
	)

	var log func(msg string, fields ...zapcore.Field)

	switch alerts.Status {
	case "firing":
		log = logger.Warn

	case "resolved":
		log = logger.Info

	default:
		log = logger.Error
	}

	log("Events received")
	writer.WriteHeader(http.StatusOK)
}
