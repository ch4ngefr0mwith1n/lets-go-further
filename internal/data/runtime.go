package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// deklarisanje custom "Runtime" tipa, koji je zapravo "int32" tip
type Runtime int32

var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")

// ova metoda treba da vraća JSON-enkodiranu vrijednost (string) u formatu ""<runtime> mins"
// ima isti potpis kao i "Marshal" metoda, pa zbog toga zadovoljava njen interfejs
// inače će da se baci sledeća greška "json: error calling MarshalJSON for type data.Runtime: invalid character 'm' after top-level value"
//
// namjerno se koristi "value receiver", a ne "pointer receiver" (func (r *Runtime) MarshalJSON())
// to znači da će ova metoda raditi i sa "Runtime" vrijednostima i sa "pointer"-ima na "Runtime" vrijednosti
// ovo nam pruža veću fleksibilnost
func (r Runtime) MarshalJSON() ([]byte, error) {
	// generisanje string-a koji sadrži "runtime" u zahtjevanom formatu
	jsonValue := fmt.Sprintf("%d mins", r)
	// kako bi bio ispravan JSON string, mora biti unutar "double quotes"
	quotedJSONValue := strconv.Quote(jsonValue)

	return []byte(quotedJSONValue), nil
}

// In this method we need to parse the JSON string in the format "<runtime> mins", convert the runtime number to an int32,
// and then assign this to the Runtime value itself.
//
// Implement a UnmarshalJSON() method on the Runtime type so that it satisfies the
// json.Unmarshaler interface.
// IMPORTANT: Because UnmarshalJSON() needs to modify the
// receiver (our Runtime type), we must use a pointer receiver for this to work
// correctly. Otherwise, we will only be modifying a copy (which is then discarded when
// this method returns).
func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
	// We expect that the incoming JSON value will be a string in the format
	// "<runtime> mins", and the first thing we need to do is remove the surrounding
	// double-quotes from this string. If we can't unquote it, then we return the
	// ErrInvalidRuntimeFormat error.
	unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	// Split the string to isolate the part containing the number.
	parts := strings.Split(unquotedJSONValue, " ")

	// Sanity check the parts of the string to make sure it was in the expected format.
	// If it isn't, we return the ErrInvalidRuntimeFormat error again.
	if len(parts) != 2 || parts[1] != "mins" {
		return ErrInvalidRuntimeFormat
	}

	// Otherwise, parse the string containing the number into an int32. Again, if this
	// fails return the ErrInvalidRuntimeFormat error.
	i, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	// Convert the int32 to a Runtime type and assign this to the receiver. Note that we
	// use the * operator to deference the receiver (which is a pointer to a Runtime
	// type) in order to set the underlying value of the pointer.
	*r = Runtime(i)

	return nil
}
