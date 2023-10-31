package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"golang.org/x/crypto/bcrypt"
	validator "greenlight.lazarmrkic.com/internal"
	"time"
)

// koristimo `json:"-"` za polja koja ne trebaju da se prikazuju u JSON output-u
type User struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  password  `json:"-"`
	Activated bool      `json:"activated"`
	Version   int       `json:"-"`
}

// "UserModel" struct omotava "connection pool"
type UserModel struct {
	DB *sql.DB
}

// "custom" tip za "password"
// sadrži "plaintext" i "hashed" verzije lozinke korisnika
// "plaintext" je *pointer* ka String-u, tako da bi mogli da razlikujemo odsustvo lozinke u "struct"-u i lozinku sa praznim stringom ("")
type password struct {
	plaintext *string
	hash      []byte
}

var (
	ErrDuplicateEmail = errors.New("duplicate email")
)

// "Set()" metoda računa "bcrypt hash" na osnovu "plaintext" lozinke
// nakon toga, čuvaće obje vrijednosti ("plaintext" & "hash") unutar "struct"-a
func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
	// nad lozinkom iz parametra će se odraditi "re-hash" sa istim "salt" & "cost" parametrima koji su u "hash" stringu sa kojim vršimo poređenje
	// na kraju se vrši poređenje te dvije vrijednosti
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))

	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}

	// ukoliko "password hash" ima "nil" vrijednost, to će biti zbog neke greške u logici naše aplikacije
	// najvjerovatnije, zbog toga što smo zaboravili da postavimo lozinku za korisnika
	// problem svakako ne bi bio u podacima koje šalje klijent, pa nema potrebe dodavati grešku u "validation map"
	// treba odraditi "panic"
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

func (m UserModel) Insert(user *User) error {
	query := `
        INSERT INTO users (name, email, password_hash, activated) 
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at, version`

	args := []any{user.Name, user.Email, user.Password.hash, user.Activated}

	// sve operacije unutar ovog konteksta (uključujući i ovaj "query") će biti prekinute ukoliko traju duže od 3 sekunde
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// "ID", "CreatedAt" i "Version" će biti generisani u bazi
	// nakon toga, oni će biti vraćeni i učitani upravo u ovaj objekat koji je proslijeđen kao parametar metode
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

// biće vraćen "User" objekat na osnovu "email"-a
// unutar tabele stoji "UNIQUE" constraint na "email" koloni
// zbog toga će ovaj SQL upit vratiti samo jedan red (ili nijedan, u tom slučaju se vraća "ErrRecordNotFound" greška)
func (m UserModel) GetByEmail(email string) (*User, error) {
	query := `
        SELECT id, created_at, name, email, password_hash, activated, version
        FROM users
        WHERE email = $1`

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

// ažuriranje detalja o određenom korisniku
// provjeravaće se "version" polje - kako bismo spriječili "race conditions" tokom ciklusa zahtjeva
// takođe, provjeravaćemo da li prilikom ažuriranja pravimo duplikat već postojećeg email-a
func (m UserModel) Update(user *User) error {
	query := `
        UPDATE users 
        SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
        WHERE id = $5 AND version = $6
        RETURNING version`

	args := []any{
		user.Name,
		user.Email,
		user.Password.hash,
		user.Activated,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (m UserModel) GetForToken(tokenScope string, tokenPlaintext string) (*User, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `
        SELECT users.id, users.created_at, users.name, users.email, users.password_hash, users.activated, users.version
        FROM users
        INNER JOIN tokens
        ON users.id = tokens.user_id
        WHERE tokens.hash = $1
        AND tokens.scope = $2 
        AND tokens.expiry > $3`

	// koristimo "[:]" operator kako bismo "tokenHash" pretvorili u "slice"
	// niz ne možemo da proslijedimo, zato što nije podržan od strane "pq driver"-a
	args := []any{tokenHash[:], tokenScope, time.Now()}

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// odgovarajuće parametre prosljeđujemo u "query", a povratne vrijednosti će biti upisane u "User" struct:
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}
