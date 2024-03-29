package data

import (
	validator "greenlight.lazarmrkic.com/internal"
	"math"
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

// ovaj "struct" će sadržati "pagination" metadada:
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
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

func (f Filters) limit() int {
	return f.PageSize
}

func (f Filters) offset() int {
	return (f.Page - 1) * f.PageSize
}

func calculateMetadata(totalRecords int, page int, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}

	return Metadata{
		CurrentPage: page,
		PageSize:    pageSize,
		FirstPage:   1,
		// recimo da imamo situaciju gdje postoji 12 "totalRecords" i gdje "pageSize" ima vrijednost 5:
		// math.Ceil(12/5) = 3
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
	}
}
