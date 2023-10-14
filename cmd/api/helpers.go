package main

import (
	"encoding/json"
	"errors"
	"github.com/julienschmidt/httprouter"
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
