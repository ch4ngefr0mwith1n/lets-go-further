package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	// pokretanje "goroutine"-a u pozadini:
	go func() {
		// kreiranje "quit" kanala koji nosi "os.Signal" vrijednosti:
		quit := make(chan os.Signal, 1)

		// "signal.Notify()" osluškuje nadolazeće "SIGINT" i "SIGTERM" signale
		// nakon toga, prosljeđuje ih ka "quit" kanalu
		// nijedan drugi signal neće biti uhvaćen i zadržaće se njegovo "default" ponašanje
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// učitavanje signala iz "quit" kanala
		// izvršavanje "goroutine"-a će biti pauzirano sve dok se signal ne primi
		s := <-quit

		// logovanje signala koji je "uhvaćen"
		// preko "String()" metode će se izvaditi naziv signala i biće ubačen u logove
		app.logger.Info("caught signal", "signal", s.String())

		// izlazak iz aplikacije sa "0" status kodom (success)
		os.Exit(0)
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	return srv.ListenAndServe()
}
