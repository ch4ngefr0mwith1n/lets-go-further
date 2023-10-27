package main

import (
	"context"
	"errors"
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

	// ČITAVA LOGIKA:
	// kad god primimo "SIGINT" ili "SIGTERM" signal, šaljemo instrukcije ka serveru da prestane sa prihvatanjem svih novih HTTP zahtjeva
	// svi zahtjevi koji su trenutno "in-flight" imaju period od 30 sekundi da se završe prije nego što se aplikacija izgasi
	//
	// kreiranje "shutdownError" kanala
	// koristićemo ga za primanje svih grešaka koje vraća "Shutdown" funkcija
	shutdownError := make(chan error)

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
		app.logger.Info("shutting down server", "signal", s.String())

		// kreiranje "context" objekta sa timeout-om od 30 sekundi:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// poziva se "Shutdown()" funkcija na našem serveru, skupa sa proslijeđenim kontekstom
		// "Shutdown()" će vratiti "nil" ukoliko je "graceful shutdown" prošao uspješno ili sa potencijalnu grešku
		// greška može da se javi tokom zatvaranja "listener"-a ili zato što se "shutdown" nije završio u roku od 30 sekundi
		// vrijednsot greške možemo da proslijedimo ka "shutdownError" kanalu
		shutdownError <- srv.Shutdown(ctx)
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	// čim se "Shutdown()" metoda pozove nad serverom, "ListenAndServe()" metoda će vratiti "http.ErrServerClosed" grešku
	// ukoliko vidimo ovu grešku, to je dobra stvar i to znači da je "graceful shutdown" otpočeo
	// ako se desi bilo koja druga greška, nju ćemo da vratimo - u suprotnom, kod se dalje izvršava
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// ukoliko uhvatimo "http.ErrServerClosed", onda želimo da uzmemo povratnu vrijednost iz "Shutdown()" funkcije sa "shutdownError" kanala
	// ukoliko je povratna vriejdnost "error", onda imamo problem sa "graceful shutdown" i trebamo da vratimo taj "error"
	err = <-shutdownError
	if err != nil {
		return err
	}

	// u ovom dijelu koda znamo da je "graceful shutdown" prošao uspješno i trebamo da logujemo "stopped server" poruku
	app.logger.Info("stopped server", "addr", srv.Addr)

	return nil

}
