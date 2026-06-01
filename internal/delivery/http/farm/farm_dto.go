package farm

import (
	"github.com/google/uuid"
)

type CreateFarmJSON struct {
	Name      string  `json:"name" validate:"required,min=3,max=100"`
	Latitude  float64 `json:"latitude" validate:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" validate:"required,min=-180,max=180"`
}

type UpdateFarmJSON struct {
	Name      string  `json:"name" validate:"required,min=3,max=100"`
	Latitude  float64 `json:"latitude" validate:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" validate:"required,min=-180,max=180"`
}

type FarmResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	LocationKey string    `json:"location_key"`
}
