package data

import (
	"database/sql"
	"errors"
)

var (
	// ovaj tip greške za sada koristimo kada "Get()" metoda pokušava da nađe film koji nije u bazi
	ErrRecordNotFound = errors.New("record not found")
	// u slučaju "data race" konflikta:
	ErrEditConflict = errors.New("edit conflict")
)

// unutar ovog "struct"-a ćemo čuvati sve modele
// imaće funkciju "container"-a i biće pogodan za našu svrhu, jer će biti dosta modela kako aplikacija bude rasla
type Models struct {
	Movies MovieModel
}

// ova metoda vraća "Models" struct koji sadrži INICIJALIZOVAN "MovieModel"
func NewModels(db *sql.DB) Models {
	return Models{
		Movies: MovieModel{DB: db},
	}
}
