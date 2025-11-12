package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Farm struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	SoilType  string    `json:"soil_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (f *Farm) Validate() error {
	if f.ID == uuid.Nil {
		return errors.New("ID is required")
	}
	if f.UserID == uuid.Nil {
		return errors.New("UserID is required")
	}
	if f.Name == "" {
		return errors.New("Name is required")
	}
	if f.Latitude < -90 || f.Latitude > 90 {
		return errors.New("Latitude must be between -90 and 90")
	}
	if f.Longitude < -180 || f.Longitude > 180 {
		return errors.New("Longitude must be between -180 and 180")
	}
	if f.SoilType == "" {
		return errors.New("SoilType is required")
	}
	return nil
}
