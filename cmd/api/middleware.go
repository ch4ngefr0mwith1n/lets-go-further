package main

import (
	"fmt"
	"golang.org/x/time/rate"
	"net"
	"net/http"
	"sync"
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

// "IP-based" rate limiter
// svaki put kad određeni klijent napravi "request" ka API-ju, inicijalizovaće se novi "rate limiter" i biće dodat u mapu
// za svaki prateći "request", izvadićemo klijentov "rate limiter" iz mape i provjeriti da li njegov "request" treba da se odobri
// to se radi pozivom "Allow()" metode
func (app *application) rateLimit(next http.Handler) http.Handler {
	var (
		// mapa koja sadrži klijentske IP adrese i "rate limiter"-e
		clients = make(map[string]*rate.Limiter)
		// mapa kao struktura podataka nije sigurna za "concurrent use"
		// "rateLimit()" middleware može da se izvršava u više "gouroutine"-a u isto vrijeme
		// takođe, Golang-ov server za svaki HTTP "request" ima poseban "goroutine"
		// zbog toga, preko "mutex"-a će se uvesti ograničenje - samo jedan "goroutine" može da čita vrijednosti iz mape ili da upisuje u nju
		mu sync.Mutex
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// vadi se klijentska IP adresa iz "request"-a
		// "_" bi predstavljao "port" iz adrese
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		//zaključavanje "mutex"-a, kako se ovaj kod ne bi izvšavao konkurentno
		mu.Lock()

		// provjera da li IP adresa postoji unutar mape
		// ukoliko ne, inicijalizuje se novi "rate limiter" i dodaje se u mapu skupa sa povezanom IP adresom
		if _, found := clients[ip]; !found {
			clients[ip] = rate.NewLimiter(2, 4)
		}

		if !clients[ip].Allow() {
			mu.Unlock()
			app.rateLimitExceededResponse(w, r)
			return
		}

	})
}
