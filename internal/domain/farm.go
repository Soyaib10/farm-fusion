package domain

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Farm struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Name        string    `json:"name"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	LocationKey string    `json:"location_key"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewLocationKey(latitude, longitude float64) string {
	return fmt.Sprintf("%.2f_%.2f", roundCoordinate(latitude), roundCoordinate(longitude))
}

func roundCoordinate(value float64) float64 {
	rounded := math.Round(value*100) / 100
	if rounded == 0 {
		return 0
	}
	return rounded
}
