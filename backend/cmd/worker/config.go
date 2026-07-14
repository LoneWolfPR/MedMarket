package main

import (
	"github.com/LoneWolfPR/MedMarket/backend/internal/envkeys"
)

type config struct {
	DatabaseURL      string
	PharmacyABaseURL string
	PharmacyASecret  string
	PharmacyBBaseURL string
	PharmacyBSecret  string
	TemporalHostPort string
}

func loadConfig() (config, error) {
	var r envkeys.Reader
	cfg := config{
		DatabaseURL:      r.Require(envkeys.DBURL),
		PharmacyABaseURL: r.Require(envkeys.PharmacyABaseURL),
		PharmacyASecret:  r.Require(envkeys.PharmacyASecret),
		PharmacyBBaseURL: r.Require(envkeys.PharmacyBBaseURL),
		PharmacyBSecret:  r.Require(envkeys.PharmacyBSecret),
		TemporalHostPort: r.Require(envkeys.TemporalHostPort),
	}
	if err := r.Err(); err != nil {
		return config{}, err
	}
	return cfg, nil
}
