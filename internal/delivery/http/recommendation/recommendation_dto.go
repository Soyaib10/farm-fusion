package recommendation

type CropRequestJSON struct {
	N           float64 `json:"N"`
	P           float64 `json:"P"`
	K           float64 `json:"K"`
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
	PH          float64 `json:"ph"`
	Rainfall    float64 `json:"rainfall"`
}

type FertilizerRequestJSON struct {
	Temperature  float64 `json:"temperature"`
	Humidity     float64 `json:"humidity"`
	SoilMoisture float64 `json:"soil_moisture"`
	SoilType     string  `json:"soil_type"`
	CropType     string  `json:"crop_type"`
	Nitrogen     float64 `json:"nitrogen"`
	Potassium    float64 `json:"potassium"`
	Phosphorous  float64 `json:"phosphorous"`
}
