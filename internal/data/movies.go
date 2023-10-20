package data

import (
	"database/sql"
	"errors"
	"github.com/lib/pq"
	validator "greenlight.lazarmrkic.com/internal"
	"time"
)

type Movie struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"`
	Runtime   Runtime   `json:"runtime,omitempty"`
	Genres    []string  `json:"genres,omitempty"`
	Version   int32     `json:"version"`
}

// "MovieModel" struct omotava "sql.DB" connection pool"
// preko njega ćemo vršiti interakciju sa bazom
// on će biti sadržan unutar "Models" struct-a
type MovieModel struct {
	DB *sql.DB
}

// "Insert" metoda prima "*Movie" pointer, pa se nakon poziva "Scan()" metode ažuriraju vrijednosti na lokaciji na koju pointer pokazuje
func (m MovieModel) Insert(movie *Movie) error {
	query := `
        INSERT INTO movies (title, year, runtime, genres) 
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at, version`

	// ovdje će biti definisane vrijednosti koje idu u "placeholder" parametre
	// BITNO:
	// niz žanrova će biti ubačen preko "pq.Array()"
	// preko ove metode možemo da ubacujemo i ostale nizove različitih tipova (bool, byte, int32, int64,...)
	args := []any{movie.Title, movie.Year, movie.Runtime, pq.Array(movie.Genres)}

	// koristi se "QueryRow()" jer nam upit vraća jedan red podataka
	// naš "INSERT" treba da vrati tri reda - "ID" / "CreatedAt" i "Version"
	return m.DB.QueryRow(query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

// koristimo "int64" iako "ID" nikada ne treba da bude negativan
// prva opcija bi bila "uint", ali PostgreSQL unutar sebe nema "unsigned integers"
func (m MovieModel) Get(id int64) (*Movie, error) {
	// "ID" je "bigserial" tipa, paće unutar baze da se odrađuje "autoincrement"
	// nikada neće imati vrijednost manju od "1"
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
        SELECT id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE id = $1`

	// unutar ovog "struct"-a će se čuvati podaci vraćeni iz baze:
	var movie Movie

	// vratiće se samo jedan red iz tabele, pa zbog toga koristimo "QueryRow":
	err := m.DB.QueryRow(query, id).Scan(
		&movie.ID,
		&movie.CreatedAt,
		&movie.Title,
		&movie.Year,
		&movie.Runtime,
		// mora da se koristi "pq.Array()", jer se skenira "text[]" niz:
		pq.Array(&movie.Genres),
		&movie.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &movie, nil
}

// prilikom ažuriranja vrijednosti za "Movie" objekat, "id" i "createdAt" ne trebaju da budu modifikovani
// klijent ne treba da pristupa "version" polju
// međutim,u našem slučaju ćemo ipak mijenjati sve navedene vrijednosti
func (m MovieModel) Update(movie *Movie) error {
	// nakon izvršavanja "query"-ja, "version" će biti uvećana za 1:
	query := `
        UPDATE movies 
        SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
        WHERE id = $5
        RETURNING version`

	// "slice" sa vrijednostima za "placeholder" parametre:
	args := []any{
		movie.Title,
		movie.Year,
		movie.Runtime,
		pq.Array(movie.Genres),
		movie.ID,
	}

	//
	// povratna vrijednost za "Version" iz query-ja će biti učitana u "Movie" struct
	// njegova vrijednost će biti izmijenjena zbog "movie *Movie" iz potpisa metoda
	return m.DB.QueryRow(query, args...).Scan(&movie.Version)
}

func (m MovieModel) Delete(id int64) error {
	return nil
}

func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")

	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")

	v.Check(movie.Runtime != 0, "runtime", "must be provided")
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")

	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}
