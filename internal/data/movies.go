package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// koristimo "int64" iako "ID" nikada ne treba da bude negativan
// prva opcija bi bila "uint", ali PostgreSQL unutar sebe nema "unsigned integers"
func (m MovieModel) Get(id int64) (*Movie, error) {
	// "ID" je "bigserial" tipa, pa će unutar baze da se odrađuje "autoincrement"
	// nikada neće imati vrijednost manju od "1"
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	// "pg_sleep" će simulirati kašnjenje pri radu sa bazom
	query := `
        SELECT id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE id = $1`

	// unutar ovog "struct"-a će se čuvati podaci vraćeni iz baze:
	var movie Movie

	// biće kreiran "context.Context" objekat koji nosi "timeout" rok od 3 sekunde
	// prazan "context.Background()" će služiti kao "parent" context
	//
	// "timeout" počinje kada se kontekst objekat kreira preko "context.WithTimeout()" metode
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	// koristimo "defer" kako bi se ugasio "context" objekat prije nego što "Get" metoda vrati vrijednost
	// bez "defer"-a, resursi neće biti oslobođeni sve dok ne dođe do "3-second-timeout"-a ili dok ne pukne "parent" context
	defer cancel()

	// ranije smo koristili "QueryRow()" metodu jer se vraća samo jedan red iz tabele
	// sada nam je potrebna "QueryRowContext()" metoda jer moramo da postavimo "context" sa "timeout"-om
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// koristi se "QueryRow()" jer nam upit vraća jedan red podataka
	// naš "INSERT" treba da vrati tri reda - "ID" / "CreatedAt" i "Version"
	return m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

// prilikom ažuriranja vrijednosti za "Movie" objekat, "id" i "createdAt" ne trebaju da budu modifikovani
// klijent ne treba da pristupa "version" polju
// međutim,u našem slučaju ćemo ipak mijenjati sve navedene vrijednosti
func (m MovieModel) Update(movie *Movie) error {
	// nakon izvršavanja "query"-ja, "version" će biti uvećana za 1
	// BITNO - DATA RACE CONDITION:
	// dešava se kada dva klijenta pokušavaju da ažuriraju isti red u isto vrijeme
	// odnosno, imaćemo dvije "goroutine" i prva treba da uspije, dok druga treba da baci "error"
	//
	// u našem slučaju, ažuriranje će biti odrađeno jedino ukoliko "version number" još uvijek ima vrijednost "N"
	// ukoliko je vrijednost u međuvremenu izmijenjena - onda se "update" neće izvršiti i klijent će dobiti "error"
	query := `
        UPDATE movies 
        SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
        WHERE id = $5 AND version = $6
        RETURNING version`

	// "slice" sa vrijednostima za "placeholder" parametre:
	args := []any{
		movie.Title,
		movie.Year,
		movie.Runtime,
		pq.Array(movie.Genres),
		movie.ID,
		movie.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// povratna vrijednost za "Version" iz query-ja će biti učitana u "Movie" struct
	// njegova vrijednost će biti izmijenjena zbog "movie *Movie" iz potpisa metoda
	//
	// ukoliko prilikom upita ne možemo da pronađemo red u tabeli, onda znamo da je on ili izbrisan ili se verzija u međuvremenu izmjenila
	// u tom slučaju vraćamo "ErrEditConflict"
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m MovieModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
        DELETE FROM movies
        WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// koristimo "Exec()" metodu jer se nakon brisanja neće vratiti nijedan red
	// međutim, ova metoda vraća "sql.Result" objekat (sadrži broj redova na koje je "query" uticao)
	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// ukoliko "query" nije uticao ni na jedan red, onda "movies" tabela nije sadržala zapis sa datim "ID"-em
	// u tom slučaju ćemo vratiti grešku
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// ova metoda će vraćati "Movie" slice
// ona će da prihvata razne "filter" parametre, iako ih na početku nećemo koristiti
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, Metadata, error) {
	// oba filtera će biti "optional" ('' ili '{}')
	// "@>" predstavlja "contained by" operator u PostgreSQL
	//
	// PostgreSQL FULL TEXT SEARCH ("Natural Language Search")
	// unutar ove metode mora biti podržana i parcijalna pretraga ("partial matches")
	// "(to_tsvector('simple', title)" - recimo, "The Breakfast Club" će biti razbijen na "breakfast", "club" i "the"
	// "plainto_tsquery('simple', $1)" - "formatted query term" koji PostgreSQL "full search" može da razumije, recimo "The Club" postaje 'the' & 'club'
	//
	// unutar "listMovies" handler-a imamo sledeće "default" vrijednosti:
	// input.Title = app.readString(qs, "title", "")
	// input.Genres = app.readCSV(qs, "genres", []string{})
	//
	// interpolacija SQL upita za sortiranje po zadatoj koloni po zadatom redoslijedu ("ASC"/"DESC")
	//
	// "LIMIT" - predstavlja maksimalan broj redova koje upit treba da vrati
	// "OFFSET" - preskače određeni broj redova prije nego što se redovi iz trenutnog upita vrate
	//
	// "window" funkcija vraća ukupan broj (isfiltriranih) redova
	query := fmt.Sprintf(`
        SELECT count(*) OVER(), id, created_at, title, year, runtime, genres, version
        FROM movies
        WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '') 
        AND (genres @> $2 OR $2 = '{}')     
        ORDER BY %s %s, id ASC
        LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// "placeholder" parametriće biti stavljeni u jedan "slice"
	args := []any{title, pq.Array(genres), filters.limit(), filters.offset()}

	// "QueryContext" metoda će vratiti "sql.Rows resultset" - koji sadrži rezultat
	// u ovu metodu će se proslijediti "variadic" parametar "args"
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		// ukoliko se desi greška, vratiće se i prazan "Metadata" struct
		return nil, Metadata{}, err
	}

	defer rows.Close()

	totalRecords := 0
	// prazan "slice" koji će sadržati filmove:
	movies := []*Movie{}

	// "rows.Next()" vrši iteraciju preko redova unutar "ResultSet"-a:
	for rows.Next() {
		// prazan "Movie" struct - kojiće sadržati podatke za individualne filmove
		var movie Movie

		// vrijednosti iz SQL reda se ubacuju u "Movie" struct
		err := rows.Scan(
			&totalRecords, // ubacuje se vrijednost "count"-a iz "window" funkcije
			&movie.ID,
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			pq.Array(&movie.Genres),
			&movie.Version,
		)

		if err != nil {
			return nil, Metadata{}, err
		}

		movies = append(movies, &movie)
	}

	// nakon završetka "rows.Next()" petlje, poziva se "rows.Err()" kako bi se vratila greška ukoliko je došlo do nje
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return movies, metadata, nil
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
