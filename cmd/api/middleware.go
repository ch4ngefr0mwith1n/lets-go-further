package main

import (
	"fmt"
	"golang.org/x/time/rate"
	"net"
	"net/http"
	"sync"
	"time"
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
	// ovaj "struct" će sadržati "rate limiter" i "last seen" za svakog klijenta
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		// mapa koja sadrži klijentske IP adrese i klijente (unutar kojih je "rate limiter")
		clients = make(map[string]*client)
		// mapa kao struktura podataka nije sigurna za "concurrent use"
		// "rateLimit()" middleware može da se izvršava u više "gouroutine"-a u isto vrijeme
		// takođe, Golang-ov server za svaki HTTP "request" ima poseban "goroutine"
		// zbog toga, preko "mutex"-a će se uvesti ograničenje - samo jedan "goroutine" može da čita vrijednosti iz mape ili da upisuje u nju
		// (a mutual exclusion lock)
		mu sync.Mutex
	)

	// "goroutine" koji se izvršava u pozadini
	// uklanja na svaki minut stare unose iz "clients" mape
	go func() {
		for {
			time.Sleep(time.Minute)

			// zaključavanje "mutex"-a
			// treba spriječiti "rate limiter" provjere sve dok se vrši "clean-up"
			mu.Lock()

			// iteracija preko svih klijenata
			// ukoliko nisu pristupali resursima u zadnja 3 minuta, onda unos treba da se izbriše iz mape
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}

			// otključavanje "mutex"-a nakon što se čišćenje završi
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "rate limit" provjera se vrši samo ako je "rate limiting" omogućen:
		if app.config.limiter.enabled {
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
			// ukoliko ne, inicijalizuje se novi klijent (unutar kog su "limiter" i "last seen") i dodaje se u mapu skupa sa povezanom IP adresom
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
				}
			}

			// ažuriranje "last seen" vremena
			clients[ip].lastSeen = time.Now()

			// za trenutnu IP adresu se poziva "Allow()" metoda
			// ukoliko "request" nije dozvoljen, "mutex" se otključava i na kraju se šalje "429 Too Many Requests" odgovor
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}

			// BITNO:
			// "mutex" treba da se otključa prije poziva narednog "handler"-a u lancu
			// ne poziva se "defer", zato što "mutex" ne bi bio otključan dok svi "handler"-i ispod ovog "middleware"-a ne bi bili otključani
			mu.Unlock()
		}

		next.ServeHTTP(w, r)
	})
}
