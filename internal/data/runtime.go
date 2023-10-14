package data

import (
	"fmt"
	"strconv"
)

// deklarisanje custom "Runtime" tipa, koji je zapravo "int32" tip
type Runtime int32

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
