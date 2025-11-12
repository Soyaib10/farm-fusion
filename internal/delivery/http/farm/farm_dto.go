package farm

import "time"

type CreateFarmRequest struct {
	UserID    string  `json:"user_id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	SoilType  string  `json:"soil_type"`
}

type UpdateFarmRequest struct {
	UserID    string  `json:"user_id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	SoilType  string  `json:"soil_type"`
}

type FarmResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	SoilType  string    `json:"soil_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
