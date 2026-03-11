package models

import "time"

// CarModel represents one collectible die-cast model in the library.
type CarModel struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	ModelCode string    `json:"modelCode"`
	Brand     string    `json:"brand"`
	Series    string    `json:"series"`
	Scale     string    `json:"scale"`
	Year      int       `json:"year"`
	Color     string    `json:"color"`
	Material  string    `json:"material"`
	Condition string    `json:"condition"`
	ImageURL  string    `json:"imageUrl"`
	Gallery   []string  `json:"gallery"`
	Notes     string    `json:"notes"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CreateModelRequest is the payload for adding a new model.
type CreateModelRequest struct {
	Name      string   `json:"name"`
	ModelCode string   `json:"modelCode"`
	Brand     string   `json:"brand"`
	Series    string   `json:"series"`
	Scale     string   `json:"scale"`
	Year      int      `json:"year"`
	Color     string   `json:"color"`
	Material  string   `json:"material"`
	Condition string   `json:"condition"`
	ImageURL  string   `json:"imageUrl"`
	Gallery   []string `json:"gallery"`
	Notes     string   `json:"notes"`
	Tags      []string `json:"tags"`
}
