package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
)

func StartPprofApiServer(ctx context.Context, httpPort int) {
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", httpPort),
	}

	slog.Info("Starting pprof api server", "addr", fmt.Sprintf(":%d", httpPort))
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("ListenAndServe failed", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down pprof api server...")
	if err := server.Shutdown(context.Background()); err != nil {
		slog.Error("Server Shutdown Failed", "error", err)
	} else {
		slog.Info("Pprof api server exited gracefully")
	}
}
