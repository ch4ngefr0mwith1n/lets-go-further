package main

import (
	"errors"
	validator "greenlight.lazarmrkic.com/internal"
	"greenlight.lazarmrkic.com/internal/data"
	"net/http"
	"time"
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

	// prilikom registracije, dodaće se "movies:read" permission za novog korisnika:
	err = app.models.Permissions.AddPermissionForUser(user.ID, "movies:read")
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// nakon što se korisnik kreira u bazi, za njega treba generisati "activation token"
	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// slanje "welcome" email-a može značajno da uveća latenciju za "request/response" ciklus
	// zbog toga, slanje email-a treba izdvojiti u zasebnu "gouroutine"-u
	// ona će da se izvršava PARALELNO sa pratećim kodom
	// odnosno, mogli bismo da vratimo HTTP response klijentu bez čekanja na slanje email-a
	// pozadinski "gorotuine" će da se izvršava još dugo nakon vraćanja JSON-a
	//
	// pokretanje "goroutine" - koja pokreće anonimnu funkciju koja šalje "welcome" email:
	app.background(func() {
		// trenutno postoji više podataka koje trebamo da proslijedimo u "email" šablone
		// zbog toga ćemo da kreiramo mapu koja će da čuva ove podatke
		data := map[string]any{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}

		// slanje "welcome" mejla, a mapa se prosljeđuje kao "dynamic" podatak:
		err = app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.Error(err.Error())
		}
	})

	// ispisivanje JSON odgovora koji sadrži podatke o korisniku, skupa sa "202 Accepted" status kodom
	// to znači da smo "request" prihvatili za procesuiranje i da se ono još uvijek nije završilo
	err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	// korisnik preko "PUT request"-a šalje "plaintext activation token
	// ukoliko pošaljemo više "PUT request"-ova jedan za drugim, onda će samo prvi da ima efekta (ukoliko je token validan)
	//
	var input struct {
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validacija "plaintext" tokena kog korisnik šalje:
	v := validator.New()

	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// vađenje detalja o korisniku za kog je vezan "plaintext" token
	// ukoliko se ne nađe nijedan korisnik, korisnik dobija povratnu informaciju da token nije validan
	user, err := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// ažuriranje "Activated" statusa za datog korisnika:
	user.Activated = true

	// čuvanje ažuriranog korisnika u bazi i provjera da li postoje neki "edit" konflikti
	// koristi se isti pristup kao za "movie" podatke
	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// ukoliko je sve prošlo kako treba, onda brišemo sve aktivacione tokene za datog korisnika:
	err = app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// slanje JSON-a sa ažuriranim podacima o korisniku:
	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
