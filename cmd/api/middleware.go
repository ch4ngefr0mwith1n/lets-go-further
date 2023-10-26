package main

import (
	"fmt"
	"golang.org/x/time/rate"
	"net/http"
)

// ukoliko se desi neki "panic", njega će automatski da uhvati "http.Server"
// on će automatski da prikaže "stack" za "goroutine", zatvori HTTP konekciju i loguje "error" i "stack trace"
// u tom slučaju će HTTP konekcija samo biti zatvorena, bez ikakvog konteksta
//
// bilo bi bolje da se pošalje neka informacija ka klijentu - odnosno "500 Internal Server Error"
// za tu svrhu će nam poslužiti "recoverPanic" middleware
func (app *application) recoverPanic(next http.Handler) http.Handler {
	// kreira se "deferred" funkcija
	// ona će uvijek da se izvrši ako se desi "panic"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// postavljenje "Connection: close" header-a na Response
				// Golang-ov "HTTP server" će automatski da zatvori konekciju nakon što se pošalje "response"
				w.Header().Set("Connection", "Close")
				// pozivom "recover()" ćemo dobiti odgovarajuću vrijednost
				// ona ima povratnu vrijednost "any", pa ćemo je normalizovati preko "fmt.Errorf()"
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()
		// pozivanje narednog "middleware"-a u lancu:
		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	// inicijalizuje se "rate limiter"
	// on dozvoljava 2 "request"-a po sekundi sa maksimalno 4 "request"-a u jednom "burst"-u
	limiter := rate.NewLimiter(2, 4)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			app.rateLimitExceededResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
