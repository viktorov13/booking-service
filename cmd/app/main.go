//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/app/main.go -o docs

package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	docs "room-booking-service/docs"
	"room-booking-service/internal/auth"
	"room-booking-service/internal/config"
	"room-booking-service/internal/db/postgres"
	"room-booking-service/internal/httpapi"
	"room-booking-service/internal/service"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// @title Room Booking Service API
// @version 1.0
// @description API for booking meeting rooms.
// @host localhost:8080
// @BasePath /
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cfg := config.Load()
	docs.SwaggerInfo.Host = "localhost:" + cfg.Port
	docs.SwaggerInfo.BasePath = "/"

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if err := postgres.RunMigrations(ctx, db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	repo := postgres.New(db)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, 24*time.Hour)
	appService := service.New(repo, jwtManager, nil)
	handler := httpapi.NewHandler(appService, jwtManager)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("server started on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown server: %v", err)
	}
}
