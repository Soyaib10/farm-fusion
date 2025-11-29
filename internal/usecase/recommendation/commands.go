package recommendation

type CropCommand struct {
	N           float64
	P           float64
	K           float64
	Temperature float64
	Humidity    float64
	PH          float64
	Rainfall    float64
}

type FertilizerCommand struct {
	Temperature  float64
	Humidity     float64
	SoilMoisture float64
	SoilType     string
	CropType     string
	Nitrogen     float64
	Potassium    float64
	Phosphorous  float64
}
