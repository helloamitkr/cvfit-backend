package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/akrylysov/algnhsa"
	"github.com/helloamitkr/cvfit-backend/internal/handler"
	"github.com/helloamitkr/cvfit-tools/awsutil"
	cvfitv1 "github.com/helloamitkr/cvfit-tools/gen/cvfit/v1"
)

func main() {
	mux := buildMux()

	// AWS_LAMBDA_FUNCTION_NAME is always set by the Lambda runtime.
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		algnhsa.ListenAndServe(mux, nil)
		return
	}

	runLocal(mux)
}

func buildMux() *http.ServeMux {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-south-1"
	}

	awsCfg, err := awsutil.LoadAWSConfig(context.Background(), region)
	if err != nil {
		slog.Warn("AWS config unavailable — DynamoDB/S3 disabled", "err", err)
	}

	var db *awsutil.DynamoDBClient
	if err == nil {
		db = awsutil.NewDynamoDBClient(awsCfg)
	}
	tableName := os.Getenv("DYNAMODB_TABLE_NAME")

	mux := http.NewServeMux()

	mux.Handle(cvfitv1.ResumeServicePathPrefix,
		withCORS(cvfitv1.NewResumeServiceServer(handler.NewResumeHandler(db, tableName))))
	mux.Handle(cvfitv1.ATSServicePathPrefix,
		withCORS(cvfitv1.NewATSServiceServer(handler.NewATSHandler())))
	mux.Handle(cvfitv1.TemplateServicePathPrefix,
		withCORS(cvfitv1.NewTemplateServiceServer(handler.NewTemplateHandler())))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"cvfit-api"}`)) //nolint:errcheck
	})

	return mux
}

func runLocal(mux *http.ServeMux) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      withRequestLog(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("CVFit API started",
			"addr", "http://localhost:"+port,
			"health", "http://localhost:"+port+"/health",
		)
		slog.Info("Twirp endpoints",
			"resume",   cvfitv1.ResumeServicePathPrefix,
			"ats",      cvfitv1.ATSServicePathPrefix,
			"template", cvfitv1.TemplateServicePathPrefix,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "err", err)
	}
	slog.Info("stopped")
}

// withCORS injects CORS headers and handles preflight requests.
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// withRequestLog logs method, path, status, and latency for every request.
func withRequestLog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		h.ServeHTTP(rw, r)
		slog.Info("req",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.code,
			"ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.code = code
	sw.ResponseWriter.WriteHeader(code)
}
