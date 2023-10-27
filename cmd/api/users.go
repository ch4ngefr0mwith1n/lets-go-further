package main

import (
	"errors"
	validator "greenlight.lazarmrkic.com/internal"
	"greenlight.lazarmrkic.com/internal/data"
	"net/http"
)

// "registerUserHandler" treba da kreira novi "User" struct, koji sadrži podatke poslate na "endpoint"
// nakon toga, treba da odradi validaciju polja
// na kraju, da proslijedi taj struct ka "UserModel.Insert()" metodi i da unos bude ubačen u bazu
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// kopiraju se vrijednosti iz "request" body u novi "User" struct
	// "Activated" polje će za sada eksplicitno da bude "false"
	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// ubacivanje "User"-a u bazu:
	err = app.models.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// ispisivanje JSON odgovora koji sadrži podatke o korisniku, skupa sa "201 Created" status kodom:
	err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
