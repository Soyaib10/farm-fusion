package domain

type CropRecommendation struct {
	Crop         string
	Confidence   float64
	Alternatives []Alternative
	Warning      string
}

type FertilizerRecommendation struct {
	Fertilizer   string
	Confidence   float64
	Alternatives []Alternative
	Warning      string
}

type Alternative struct {
	Name       string
	Confidence float64
}
