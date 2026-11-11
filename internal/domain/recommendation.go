package domain

// Allowed SoilType and CropType categorical values supported by the ML fertilizer model.
var (
	AllowedSoilTypes = []string{"Sandy", "Loamy", "Black", "Red", "Clayey"}
	AllowedCropTypes = []string{
		"Maize", "Sugarcane", "Cotton", "Tobacco", "Paddy",
		"Barley", "Wheat", "Millets", "Oil seeds", "Pulses", "Ground Nuts",
	}
)

// CropQuery holds soil and climate measurements used to predict optimal crops.
type CropQuery struct {
	N           float64
	P           float64
	K           float64
	Temperature float64
	Humidity    float64
	PH          float64
	Rainfall    float64
}

// FertilizerQuery holds soil, crop, and nutrient measurements used to predict optimal fertilizer.
type FertilizerQuery struct {
	Temperature  float64
	Humidity     float64
	SoilMoisture float64
	SoilType     string
	CropType     string
	Nitrogen     float64
	Potassium    float64
	Phosphorous  float64
}

// CropRecommendation contains the predicted optimal crop and top alternatives.
type CropRecommendation struct {
	Crop         string        `json:"crop"`
	Confidence   float64       `json:"confidence"`
	Alternatives []Alternative `json:"alternatives"`
	Warning      string        `json:"warning,omitempty"`
}

// FertilizerRecommendation contains the predicted optimal fertilizer and top alternatives.
type FertilizerRecommendation struct {
	Fertilizer   string        `json:"fertilizer"`
	Confidence   float64       `json:"confidence"`
	Alternatives []Alternative `json:"alternatives"`
	Warning      string        `json:"warning,omitempty"`
}

// Alternative represents a candidate prediction and its probability score.
type Alternative struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}
