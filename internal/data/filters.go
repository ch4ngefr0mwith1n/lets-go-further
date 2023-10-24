package data

import (
	validator "greenlight.lazarmrkic.com/internal"
	"strings"
)

// "page" / "page_size" i "sort" su parametri koje ćemo koristiti i na drugim "endpoint"-ovima
// zbog toga ćemo ih staviti u "Filters" struct
type Filters struct {
	Page     int
	PageSize int
	Sort     string
	// dodajemo podržane vrijednosti za sortiranje ("id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime")
	SortSafeList []string
}

func ValidateFilters(v *validator.Validator, f Filters) {
	// "page" i "pageSize" parametri treba da budu u normalnim granicama:
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a maximum of 10 million")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")

	// provjera da li se "sort" parametri nalaze u okviru "safe list" vrijednosti
	v.Check(validator.PermittedValue(f.Sort, f.SortSafeList...), "sort", "invalid sort value")
}

// "sortColumn()" i "sortDirection()" helper metode transformišu "query string" vrijednost (recimo "-year") u vrijednosti koje možemo da koristimo unutar SQL "query"-ja
//
// provjera da li klijentsko "Sort" polje postoji unutar "safelist"
// ukoliko postoji, onda se vadi naziv kolone iz "Sort" polja i uklanja se "-" karakter (ukoliko on postoji)
func (f Filters) sortColumn() string {
	for _, safeValue := range f.SortSafeList {
		if f.Sort == safeValue {
			return strings.TrimPrefix(f.Sort, "-")
		}
	}

	panic("unsafe sort parameter: " + f.Sort)
}

// vraća se smjer sortiranja ("ASC" ili "DESC"), u zavisnosti od "prefix" karaktera unutar "Sort" polja
func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}

	return "ASC"
}
