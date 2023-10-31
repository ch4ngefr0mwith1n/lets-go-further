package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

// povratna vrijednost mora da bude "http.Handler":
func (app *application) routes() http.Handler {
	router := httprouter.New()

	// "router" takođe treba da šalje grešku ukoliko ne može da pronađe odgovarajuću rutu
	// tu trebamo da stavimo iste "JSON response"-ove, ali oni moraju da zadovoljavaju "http.Handler" interfejs
	// zbog toga, mora da se odradi konverzije
	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	// sada moramo sa "middleware"-om da "omotamo" svaku rutu:
	router.HandlerFunc(http.MethodGet, "/v1/movies", app.listMoviesHandler)
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)
	router.HandlerFunc(http.MethodPost, "/v1/movies", app.createMovieHandler)
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.showMovieHandler)
	// ukoliko radimo "partial update", onda trebamo da koristimo "PATCH":
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.updateMovieHandler)
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.deleteMovieHandler)

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/activation", app.createActivationTokenHandler)
	return app.recoverPanic(app.rateLimit(router))
}
