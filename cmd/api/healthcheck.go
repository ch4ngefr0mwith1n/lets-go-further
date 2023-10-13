package main

import (
	"fmt"
	"net/http"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	js := `{"status": "available", "environment": %q, "version": %q}`
	js = fmt.Sprintf(js, app.config.env, version)

	// Golang po default-u šalje "text/plain; charset=utf-8":
	w.Header().Set("Content-Type", "application/json")
	// upisivanje JSON-a u HTTP response:
	w.Write([]byte(js))
}
