package data

// "page" / "page_size" i "sort" su parametri koje ćemo koristiti i na drugim "endpoint"-ovima
// zbog toga ćemo ih staviti u "Filters" struct
type Filters struct {
	Page     int
	PageSize int
	Sort     string
}
