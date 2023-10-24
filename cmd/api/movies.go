package main

import (
	"errors"
	"fmt"
	validator "greenlight.lazarmrkic.com/internal"
	"greenlight.lazarmrkic.com/internal/data"
	"net/http"
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

	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		// ukoliko se desi "data.ErrRecordNotFound", onda šaljemo "404 Not Found" ka klijentu
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// ubacivanje "envelope{"movie": movie}" instance:
	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// "updateMovie()" handler će biti ažuriran tako da podržava "djelimične" apdejte vezane za "movie" zapise
// ovaj proces je malo komplikovaniji od "complete replacement" pristupa ranije
func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// vađenje već postojećeg "movie"-a iz baze:
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// How do we tell the difference between:
	// A client providing a key/value pair which has a zero-value value — like {"title": ""} — in which case we want to return a validation error.
	// A client not providing a key/value pair in their JSON at all — in which case we want to ‘skip’ updating the field but not send a validation error.
	//
	// "input" struct čuva podatke koji se očekuju od klijenta
	// kako bismo izbjegli rad sa "default" vrijednostima tipova (recimo, "" za stringove / "0" za brojne tipove itd.)
	// uvešćemo "pointer"-e kao tipove jer je njihova "default" vrijednost "null"
	// ukoliko klijent šalje određeni "key:value" par preko JSON-a, onda samo provjerimo da li je odgovarajuće polje
	// unutar "input" struct-a "nil" ili nije
	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"` // nizove nema potrebe da modifikujemo, oni su referentni tip
	}

	// upisivanje vrijednosti iz klijentskog JSON-a u "input" struct:
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// kopiranje vrijednosti iz "request body"-ja u odgovarajuća polja "movie" zapisa iz baze
	// ukoliko bilo koje od navedenih polja ima "nil" vrijednost, onda preskačemo upisivanje vrijednosti
	// BITNO:
	// pošto radimo sa "pointer"-ima, prvo moramo da ih dereferenciramo preko "*" operatora
	if input.Title != nil {
		movie.Title = *input.Title
	}
	if input.Year != nil {
		movie.Year = *input.Year
	}
	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		movie.Genres = input.Genres // nema potrebe da derefenciramo "slice"
	}

	// validacija ažuriranog zapisa iz baze
	// ukoliko ona ne prođe - biće poslat "422 Unprocessable Entity" odgovor
	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// prosljeđivanje ažuriranog zapisa u "update()" metodu, sada on treba nanovo da se sačuva u bazu:
	err = app.models.Movies.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)

		}
		return
	}

	// vraćanje ažuriranog zapisa u vidu JSON odgovora:
	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.models.Movies.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "movie successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	// unutar ovog "struct"-a će se čuvati očekivane vrijednosti iz "request query string"-a:
	var input struct {
		Title  string
		Genres []string
		// ubacivanje "Filters" struct-a ("page" / "page_size" i "sort")
		data.Filters
	}

	v := validator.New()
	// "url.Values" mapa, koja sadrži "query string" podatke
	qs := r.URL.Query()

	input.Title = app.readString(qs, "title", "")
	input.Genres = app.readCSV(qs, "genres", []string{})
	// podrazumijevana vrijednost za "page_value" je 1, a za "page_size" je 20
	// treći argument koji prosljeđujemo je instanca "validator"-a
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	// podrazumijevana vrijednost za sortiranje je "id" (ascending sortiranje preko "movie ID"-a)
	input.Filters.Sort = app.readString(qs, "sort", "id")
	// dodavanje podržanih "sort" vrijednosti za ovaj "endpoint"
	input.Filters.SortSafeList = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

	// validacija nad "Filters" struct-om i provjera da li ima grešaka u "Validator" instanci
	// ukoliko se pronađu greške, biće poslat odgovor sa njihovim sadržajem
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// ubacivanje sadržaja "input" struct-a u HTTP response
	fmt.Fprintf(w, "%+v\n", input)
}
