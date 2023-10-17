package main

import (
	"fmt"
	validator "greenlight.lazarmrkic.com/internal"
	"greenlight.lazarmrkic.com/internal/data"
	"net/http"
	"time"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	// deklarisanje anonimnog "struct"-a, u njega će se dekodirati "HTTP request body":
	var input struct {
		// "tag"-ovi moraju da se poklapaju sa poljima u JSON-u:
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres  []string     `json:"genres"`
	}

	// stari pristup za dekodiranje:
	// err := json.NewDecoder(r.Body).Decode(&input)
	// novi pristup:
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// kopiranje vrijednosti iz "input" struct-a u "movie" struct, kako bismo pristupili metodi za validaciju:
	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}

	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// nakon poziva "Insert()" metode, takođe se prosljeđuje "pointer" ka validiranom "movie" struct-u
	// ovo će kreirati novi upis u bazu, a biće odrađeno i AŽURIRANJE tri polja unutar struct-a sa generisanim informacijama
	err = app.models.Movies.Insert(movie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// kada šaljemo HTTP response, onda unutar njega šaljemo i "Location" header
	// unutar njega će biti URL na kom mogu da nađu resurs koji je upravo kreiran
	// prvo se pravi prazna HTTP "header" mapa, a nakon toga dodajemo novi "Location" header
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%id", movie.ID))

	// sada se upisuju vrijednosti u JSON odgovor sa HTTP 201 kodom, skupa sa "Location" header-om:
	err = app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
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
