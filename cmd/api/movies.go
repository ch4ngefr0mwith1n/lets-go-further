package main

import (
	"encoding/json"
	"fmt"
	"greenlight.lazarmrkic.com/internal/data"
	"net/http"
	"time"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	// deklarisanje anonimnog "struct"-a, u njega će se dekodirati "HTTP request body":
	var input struct {
		// "tag"-ovi moraju da se poklapaju sa poljima u JSON-u:
		Title   string   `json:"title"`
		Year    int32    `json:"year"`
		Runtime int32    `json:"runtime"`
		Genres  []string `json:"genres"`
	}

	// dekodiranje:
	// prvo se "HTTP request body" učitava, a nakon toga se preko "Decode()" metode ubacuje u "struct"
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		app.errorResponse(w, r, http.StatusBadRequest, err.Error())
		return
	}

	fmt.Fprintf(w, "%+v\n")
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	// vrijednost parametra ćemo vaditi preko "helper" metode
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	movie := data.Movie{
		ID:        id,
		CreatedAt: time.Now(),
		Title:     "Casablanca",
		Runtime:   102,
		Genres:    []string{"drama", "romance", "war"},
		Version:   1,
	}

	// ubacivanje "envelope{"movie": movie}" instance:
	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
