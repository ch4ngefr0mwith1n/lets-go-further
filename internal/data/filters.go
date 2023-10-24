package data

import validator "greenlight.lazarmrkic.com/internal"

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
