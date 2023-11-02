package main

import (
	"context"
	"greenlight.lazarmrkic.com/internal/data"
	"net/http"
)

type contextKey string

// "user" string ćemo konvertovati u "contextKey" tip i dodijelićemo ga ovoj konstanti
// ona će nam služiti kao "key" za "getting"/"setting" informacija o korisniku unutar "request context"-a
const userContextKey = contextKey("user")

// metoda "contextSetUser()" vraća novu kopiju "request"-a, skupa sa "User" struct-om proslijeđenim iz metode:
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
	// kreiranje modifikovane kopije i dodavanje "User"-a u nju:
	ctx := context.WithValue(r.Context(), userContextKey, user)
	// postavljanje kopije na mjesto starog "context"-a:
	return r.WithContext(ctx)
}

// vađenje "User" struct-a iz "request context"-a
// ovu metodu ćemo koristiti samo na mjestima gdje očekujemo "User" vrijednost unutar konteksta
// ukoliko ne bude pronađena, baciće se "unexpected error"
func (app *application) contextGetUser(r *http.Request) *data.User {
	// vadi se vrijednost vezana za "userContextKey" ključ
	// pošto je ta vrijednost po default-u "any" tipa, mora da se izvrši konverzija u pointer "User" tip
	// "ok" je indikacija da je konverzija prošla u redu
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user value in request context")
	}

	return user
}
