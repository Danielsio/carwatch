package model

import "time"

type RawListing struct {
	Token            string
	Manufacturer     string
	ManufacturerID   int
	Model            string
	ModelID          int
	SubModel     string
	Year         int
	Month        int
	EngineVolume float64
	HorsePower   int
	EngineType   string
	GearBox      string
	Km           int
	Hand         int
	Price        int
	City         string
	Area         string
	Description  string
	ImageURL     string
	PageLink     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ScoreInfo struct {
	Score       int
	MedianPrice int
	CohortSize  int
}

type FitnessDim struct {
	Name   string
	Score  float64
	Weight float64
}

type Listing struct {
	RawListing
	SearchName       string
	DealScore        *ScoreInfo
	FitnessScore     float64
	FitnessBreakdown []FitnessDim
}
