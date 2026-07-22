package main

import (
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/envkeys"
)

const defaultJWTTTL = 24 * time.Hour
const defaultOfferTTL = time.Hour

type config struct {
	DatabaseURL         string
	JWTSecret           []byte
	JWTTTL              time.Duration
	Port                string
	PharmacyABaseURL    string
	PharmacyASecret     string
	PharmacyBBaseURL    string
	PharmacyBSecret     string
	MinioEndpoint       string
	MinioPublicEndpoint string
	MinioRegion         string
	MinioAccessKey      string
	MinioSecretKey      string
	MinioBucket         string
	MinioUseSSL         bool
	TemporalHostPort    string
	OfferTTL            time.Duration
	WebhookBaseURL      string
	OTLPEndpoint        string
}

func loadConfig() (config, error) {
	var r envkeys.Reader
	cfg := config{
		DatabaseURL:         r.Require(envkeys.DBURL),
		JWTSecret:           []byte(r.Require(envkeys.JWTSecret)),
		JWTTTL:              r.DurationOr(envkeys.JWTTTL, defaultJWTTTL),
		Port:                r.Optional(envkeys.Port, "8080"),
		PharmacyABaseURL:    r.Require(envkeys.PharmacyABaseURL),
		PharmacyASecret:     r.Require(envkeys.PharmacyASecret),
		PharmacyBBaseURL:    r.Require(envkeys.PharmacyBBaseURL),
		PharmacyBSecret:     r.Require(envkeys.PharmacyBSecret),
		MinioEndpoint:       r.Require(envkeys.MinioEndpoint),
		MinioPublicEndpoint: r.Require(envkeys.MinioPublicEndpoint),
		MinioRegion:         r.Optional(envkeys.MinioRegion, "us-east-1"),
		MinioAccessKey:      r.Require(envkeys.MinioAccessKey),
		MinioSecretKey:      r.Require(envkeys.MinioSecretKey),
		MinioBucket:         r.Require(envkeys.MinioBucket),
		MinioUseSSL:         r.RequireBool(envkeys.MinioUseSSL),
		TemporalHostPort:    r.Require(envkeys.TemporalHostPort),
		OfferTTL:            r.PositiveDurationOr(envkeys.OfferTTL, defaultOfferTTL),
		WebhookBaseURL:      r.Require(envkeys.WebhookBaseURL),
		OTLPEndpoint:        r.Require(envkeys.OTLPEndpoint),
	}
	if err := r.Err(); err != nil {
		return config{}, err
	}
	return cfg, nil
}
