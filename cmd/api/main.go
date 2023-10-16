package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	// importuje se "pq" drajver, kako bi bio registrovan sa "database/sql" paketom
	// underscore "_" se koristi da se Golang kompajler ne bi žalio
	_ "github.com/lib/pq"
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
	// "db" struct polje će sadržati podešavanja konfiguracije za "database connection pool"
	// za sada će da sadrži samo "DSN", koji će moći da se učita sa komandne linije
	db struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
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
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")

	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// kreiranje "connection pool"-a:
	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	// "connection pool" će biti zatvoren prije izlaska iz "main" funkcije
	defer db.Close()
	logger.Info("database connection pool established")

	app := application{
		config: cfg,
		logger: logger,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", cfg.port),
		Handler:      app.routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  time.Minute,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "addr", srv.Addr, "env", cfg.env)

	err = srv.ListenAndServe()
	logger.Error(err.Error())
	os.Exit(1)
}

func openDB(cfg config) (*sql.DB, error) {
	// otvaranje praznog "connection pool"-a:
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	// Set the maximum number of open (in-use + idle) connections in the pool.
	// Note that passing a value less than or equal to 0 will mean there is no limit.
	db.SetMaxOpenConns(cfg.db.maxOpenConns)

	// Set the maximum number of idle connections in the pool.
	// Again, passing a value less than or equal to 0 will mean there is no limit.
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	// Set the maximum idle timeout for connections in the pool.
	// Passing a duration less than or equal to 0 will mean that connections are not closed due to their idle time.
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)

	// kreira se "context" vezan uz bazu
	// imaće "timeout" of 5 sekundi
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// preko "PingContext()" metode ćemo uspostaviti novu konekciju ka bazi
	// ukoliko se konekcija ne uspostavi u roku od 5 sekundi, vratiće se greška
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	// vraćanje "sql.DB" connection pool-a:
	return db, nil
}
