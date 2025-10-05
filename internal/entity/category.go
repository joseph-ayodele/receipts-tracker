package entity

// Category represents a category for data transfer between layers.
type Category struct {
	ID           int32  `json:"id"`
	Name         string `json:"name"`
	CategoryType string `json:"category_type"`
}
