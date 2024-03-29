package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	validator "greenlight.lazarmrkic.com/internal"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	// Use http.MaxBytesReader() to limit the size of the request body to 1MB.
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	// Initialize the json.Decoder, and call the DisallowUnknownFields() method on it before decoding.
	// This means that if the JSON from the client now includes any field which cannot be mapped to the target destination,
	// the decoder will return an error instead of just ignoring the field.
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	// Decode the request body into the target destination.
	err := dec.Decode(dst)
	if err != nil {
		// If there is an error during decoding, start the triage...
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

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

		// If the JSON contains a field which cannot be mapped to the target destination
		// then Decode() will now return an error message in the format "json: unknown field "<name>"".
		// We check for this, extract the field name from the error, and interpolate it into our custom error message.
		// Note that there's an open issue at https://github.com/golang/go/issues/29035 regarding turning this
		// into a distinct error type in the future.
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		// Use the errors.As() function to check whether the error has the type  *http.MaxBytesError.
		// If it does, then it means the request body exceeded our size limit of 1MB and we return a clear error message.
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)

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

	// Call Decode() again, using a pointer to an empty anonymous struct as the destination.
	// If the request body only contained a single JSON value this will return an io.EOF error.
	// So if we get anything else, we know that there is  additional data in the request body and we return our own custom error message.
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

// metoda koja vraća "string" vrijednost iz "query string"-a ili vraća "default" vrijednost
// "qs" predstavlja "query string"
func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	// vrijednost koja je uvezana sa ključem mape ("key" parametar)
	s := qs.Get(key)
	// ukoliko on ne postoji, vraća se prazan string
	if s == "" {
		return defaultValue
	}

	return s
}

// ova metoda učitava neku string vrijednost iz "query string"-a, a nakon toga odrađuje "split" preko "," karaktera
// rezultat će biti "slice"
func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)
	if csv == "" {
		return defaultValue
	}

	return strings.Split(csv, ",")
}

// ova metoda učitava "string" vrijednost iz "query string"-a i pretvara u "integer"
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		// dodatak - ukoliko konverzija iz "string" u "int" ne uspije, dodaće se "error message" u "Validator" instanci
		v.AddError(key, "must be an integer value")
		return defaultValue
	}

	return i
}

// ova funkcija služi za "panic recovery"
// ona koristi "recover()" da uhvati svaki "panic" i da izvrši logovanje "error" poruke umjesto direktnog gašenja aplikacije
func (app *application) background(fn func()) {
	// pokretanje "goroutine" u pozadini:
	go func() {
		// "recover" proces za svaki "panic"
		defer func() {
			if err := recover(); err != nil {
				app.logger.Error(fmt.Sprintf("%v", err))
			}
		}()
		// izvršavanje proizvoljne funkcije iz parametra:
		fn()
	}()
}
