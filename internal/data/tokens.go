package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	validator "greenlight.lazarmrkic.com/internal"
	"time"
)

// konstante za svrhu tokena
// za sada će postojati samo "activation" token
const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
)

// ovaj "struct" sadrži podatke za individualni token
// odnosno, "plaintext" i "hashed" verzije tokena, pripadajući User ID, vrijeme isteka tokena i svrhu (activation, authentication, itd...)
// biće dodati i odgovarajući "struct" tagovi, kako bismo kontrolisali na koji način se "struct" pojavljuje prilikom enkodiranja u JSON
type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

type TokenModel struct {
	DB *sql.DB
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	// "ttl" - "time to live"
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	// prvo se kreira "zero-byted" slice sa entropijom od 16 bajtova:
	randomBytes := make([]byte, 16)

	// nakon toga, koristi se "Read()" funkcija iz "crypto/rand" paketa da se "byte slice" popuni sa random bajtovima
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	// "byte slice" se enkodira u "base-32-encoded" string i dodjeljuje polju "Plaintext" unutar tokena
	// ovo će biti token string kog šaljemo korisniku u okviru email-a
	// biće u ovom ili sličnom formatu - "Y3QMGX3PJ3WLRL2YRTQGQ6KRHU"
	//
	// za tokene nam nije potreban "padding" karakter, pa je zbog toga odabrano "base32.NoPadding"
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	// generisanje SHA-256 "hash"-a i njega ćemo da čuvamo u "hash" polju tabele
	// "sha256.Sum256()" funkcija vraća "array" sa dužinom 32
	hash := sha256.Sum256([]byte(token.Plaintext))
	// zbog toga čemo odraditi konverziju u "slice" prije čuvanja u tabelu:
	token.Hash = hash[:]

	return token, nil
}

// "plaintext" token mora da postoji i treba da bude sa dužinom od 26 bajtova:
func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

// "New()" metoda predstavlja prečicu kojom kreiramo novi "Token" struct i nakon toga se podaci ubacuju u "tokens" tabelu
func (m TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.Insert(token)
	return token, err
}

// "Insert()" metoda u tabelu dodaje podatke za tačno određeni token:
func (m TokenModel) Insert(token *Token) error {
	query := `
        INSERT INTO tokens (hash, user_id, expiry, scope) 
        VALUES ($1, $2, $3, $4)`

	args := []any{token.Hash, token.UserID, token.Expiry, token.Scope}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, args...)
	return err
}

// ova metoda briše sve tokene određene svrhe, vezane za određenog korisnika:
func (m TokenModel) DeleteAllForUser(scope string, userID int64) error {
	query := `
        DELETE FROM tokens 
        WHERE scope = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, scope, userID)
	return err
}
