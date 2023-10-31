package main

import (
	"errors"
	validator "greenlight.lazarmrkic.com/internal"
	"greenlight.lazarmrkic.com/internal/data"
	"net/http"
	"time"
)

func (app *application) createActivationTokenHandler(w http.ResponseWriter, r *http.Request) {
	// parsiranje i validacija korisnikove "email" adrese:
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// potraga za odgovarajućim korisnikom preko "email" adrese
	// ukoliko ne možemo da ga nađemo, onda se vraća "error" ka klijentu
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// ukoliko je korisnik već aktiviran, treba vratiti grešku:
	if user.Activated {
		v.AddError("email", "user has already been activated")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// u suprotnom, treba kreirati novi aktivacioni token:
	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// slanje mejla korisniku, skupa sa dodatnim aktivacionim tokenom:
	app.background(func() {
		data := map[string]any{
			"activationToken": token.Plaintext,
		}

		// mejl adresa može da bude CASE-SENSITIVE, pa zbog toga koristimo korisnikovu mejl adresu sačuvanu u bazi
		// (a ne "input.Email" adresu koju šalje klijent u ovom slučaju)
		err := app.mailer.Send(user.Email, "token_activation.tmpl", data)
		if err != nil {
			app.logger.Error(err.Error())
		}
	})

	// slanje "202 Accepted" odgovora i odgovarajuće poruke ka klijentu:
	env := envelope{"message": "an email will be sent to you containing activation instructions"}

	err = app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	// parsiranje mejla i lozinke iz "request body":
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validacija mejla i lozinke koju šalje klijent:
	v := validator.New()

	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// potraga za odgovarajućim korisnikom preko "email" adrese
	// ukoliko ne možemo da ga nađemo, onda se vraća "401 Unauthorized" sa odgovarajućom porukom ka klijentu
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// provjera da li se lozinke poklapaju:
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// ukoliko se lozinke ne poklapaju, opet vraćamo "401 Unauthorized" ka klijentu:
	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	// ukoliko se lozinke poklapaju, onda generišemo novi "authentication" token koji traje 24 sata:
	token, err := app.models.Tokens.New(user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// token se enkodira u JSON i šalje unutar odgovora, skupa sa "201 Created" status kodom:
	err = app.writeJSON(w, http.StatusCreated, envelope{"authentication_token": token}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
