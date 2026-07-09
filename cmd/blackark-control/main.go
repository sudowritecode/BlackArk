package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sudowritecode/BlackArk/internal/config"
	"github.com/sudowritecode/BlackArk/internal/control"
	"github.com/sudowritecode/BlackArk/internal/database"
)

func main() {
	cfg := config.Load()
	if err := cfg.ValidateControl(); err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		log.Print("migrations applied")
		return
	}
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: control.New(db, cfg.APIToken, cfg.DashboardEnabled), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("control plane listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-stop.Done()
	shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	_ = srv.Shutdown(shutdown)
}
