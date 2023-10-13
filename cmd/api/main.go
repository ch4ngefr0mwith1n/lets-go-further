package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// broj verzije za aplikaciju
// za sada će biti postavljan manuelno, a u budućnosti automatski
const version = "1.0.0"

// konfiguracija za našu aplikaciju
type config struct {
	// "network port" na kom server "osluškuje" zahtjeve
	port int
	// trenutni "environment" koji aplikacija koristi (development, staging, production,...)
	// ovo će biti učitavano sa komandne linije
	env string
}

type application struct {
	config config
	logger *slog.Logger
}

func main() {
	var cfg config

	// postavljanje "default" vrijednosti za "command line flag"-ove
	// ako vrijednosti budemo unosili ručno, onda će se biti učitane u "cfg"
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	app := application{
		config: cfg,
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/healthcheck", app.healthcheckHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", cfg.port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  time.Minute,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "addr", srv.Addr, "env", cfg.env)

	err := srv.ListenAndServe()
	logger.Error(err.Error())
	os.Exit(1)
}
