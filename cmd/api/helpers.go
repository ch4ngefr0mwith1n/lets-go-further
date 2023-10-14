package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io"
	"net/http"
	"strconv"
)

func (app *application) readIDParam(r *http.Request) (int64, error) {
	// kada "httprouter" parsira "request", onda će URL parametri biti sačuvani u "request context"-u
	// preko "ParamsFromContext()" metode vadimo "slice" koji sadrži nazive parametara i njihove vrijednosti
	params := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

// "envelope" pristup sa JSON-om, ukoliko želimo da prikažemo "parent" objekat kao "top-level" u JSON-u:
//
//	{
//	   "movie": {
//	       "id": 123,
//	       "title": "Casablanca",
//	       "runtime": 102,
//	       "genres": [
//	           "drama",
//	           "romance",
//	           "war"
//	       ],
//	       "version":1
//	   }
//	}
type envelope map[string]any

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	// podaci se enkodiraju u JSON (samo "json.Marshal" je dovoljan za ovo)
	// prilikom formatiranja, koristiće se samo "tab indent" za svaki element (neće biti "line prefix"-a)
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	// dodavanje novog reda, ali čisto radi kozmetike u terminalu:
	js = append(js, '\n')

	// prelaz preko "header" mape iz parametra i dodavanje svakog "header"-a u "http.ResponseWriter header" mapu:
	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// Decode the request body into the target destination.
	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {
		// If there is an error during decoding, start the triage...
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		// "errors.Is()" se koristi za tačno određenu grešku unutar lanca
		// "errors.As()" se koristi za provjeru unutar tipa greške
		switch {
		// Use the errors.As() function to check whether the error has the type "*json.SyntaxError".
		// If it does, then return a plain-english error message which includes the location of the problem.
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		// In some circumstances Decode() may also return an io.ErrUnexpectedEOF error for syntax errors in the JSON.
		// So we check for this using errors.Is() and return a generic error message.
		// There is an open issue regarding this at https://github.com/golang/go/issues/25956.
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		// Likewise, catch any *json.UnmarshalTypeError errors.
		// These occur when the JSON value is the wrong type for the target destination.
		// If the error relates to a specific field, then we include that in our error message to make it easier for the client to debug.
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		// An io.EOF error will be returned by Decode() if the request body is empty.
		// We check for this with errors.Is() and return a plain-english error message instead.
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		// A json.InvalidUnmarshalError error will be returned if we pass something that is not a non-nil pointer to Decode().
		// We catch this and panic, rather than returning an error to our handler.
		// At the end of this chapter we'll talk about panicking versus returning errors, and discuss why it's an
		// appropriate thing to do in this specific situation.
		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		// For anything else, return the error message as-is.
		default:
			return err
		}

	}

	return nil
}
