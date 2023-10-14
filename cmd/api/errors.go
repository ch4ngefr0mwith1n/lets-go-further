package main

import (
	"fmt"
	"net/http"
)

// u slučaju neke greške na serveru, trenutno šaljemo "plain-text" greške iz "http.Error()" i "http.NotFound()" funkcija
// ova metoda će da loguje grešku, skupa sa tipom "request" metode i URL-om
func (app *application) logError(r *http.Request, err error) {
	var method = r.Method
	var uri = r.URL.RequestURI()

	app.logger.Error(err.Error(), "method", method, "uri", uri)
}

// ova metoda će da šalje "JSON-formatted" error poruke ka klijentu, skupa sa status kodom
// parametar za poruku će biti "any" tipa, zato što nam pruža veću fleksibilnost
func (app *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	env := envelope{"error": message}

	// "response" će biti generisan preko "writeJSON()" metode
	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(500)
	}
}

// ovu metodu koristimo u slučajevima kada aplikacija naiđe na neočekivan problem prilikom "runtime"-a
// unutar nje ćemo koristiti obje metode definisane iznad, radi detaljnog prikaza greške
func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)

	message := "the server has encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}

// ova metoda se koristi za slanje "404 Not Found", skupa sa JSON "error" porukom:
func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}

// ova metoda se koristi za slanje "405 Method Not Allowed", skupa sa JSON "error" porukom:
func (app *application) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	app.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

// ova metoda se koristi za slanje "400 Bad Request", skupa sa JSON "error" porukom:
func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}