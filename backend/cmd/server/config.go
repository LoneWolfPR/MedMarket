package main

import (
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/envkeys"
)

const defaultJWTTTL = 24 * time.Hour
const defaultOfferTTL = time.Hour

type config struct {
	DatabaseURL            string
	JWTSecret              []byte
	JWTTTL                 time.Duration
	Port                   string
	PharmacyABaseURL       string
	PharmacyASecret        string
	PharmacyBBaseURL       string
	PharmacyBSecret        string
	MinioEndpoint          string
	MinioPublicEndpoint    string
	MinioRegion            string
	MinioAccessKey         string
	MinioSecretKey         string
	MinioBucket            string
	MinioUseSSL            bool
	TemporalHostPort       string
	OfferTTL               time.Duration
	WebhookBaseURL         string
	OTLPEndpoint           string
	FileStorageBackend     string
	GCSBucket              string
	GCSServiceAccountEmail string
}

func loadConfig() (config, error) {
	var r envkeys.Reader
	cfg := config{
		DatabaseURL:        r.Require(envkeys.DBURL),
		JWTSecret:          []byte(r.Require(envkeys.JWTSecret)),
		JWTTTL:             r.DurationOr(envkeys.JWTTTL, defaultJWTTTL),
		Port:               r.Optional(envkeys.Port, "8080"),
		PharmacyABaseURL:   r.Require(envkeys.PharmacyABaseURL),
		PharmacyASecret:    r.Require(envkeys.PharmacyASecret),
		PharmacyBBaseURL:   r.Require(envkeys.PharmacyBBaseURL),
		PharmacyBSecret:    r.Require(envkeys.PharmacyBSecret),
		TemporalHostPort:   r.Require(envkeys.TemporalHostPort),
		OfferTTL:           r.PositiveDurationOr(envkeys.OfferTTL, defaultOfferTTL),
		WebhookBaseURL:     r.Require(envkeys.WebhookBaseURL),
		OTLPEndpoint:       r.Require(envkeys.OTLPEndpoint),
		FileStorageBackend: r.Optional(envkeys.FileStorageBackend, "minio"),
	}
	switch cfg.FileStorageBackend {
	case "gcs":
		cfg.GCSBucket = r.Require(envkeys.GCSBucket)
		cfg.GCSServiceAccountEmail = r.Require(envkeys.GCSServiceAccountEmail)
	case "minio":
		cfg.MinioEndpoint = r.Require(envkeys.MinioEndpoint)
		cfg.MinioPublicEndpoint = r.Require(envkeys.MinioPublicEndpoint)
		cfg.MinioRegion = r.Optional(envkeys.MinioRegion, "us-east-1")
		cfg.MinioAccessKey = r.Require(envkeys.MinioAccessKey)
		cfg.MinioSecretKey = r.Require(envkeys.MinioSecretKey)
		cfg.MinioBucket = r.Require(envkeys.MinioBucket)
		cfg.MinioUseSSL = r.RequireBool(envkeys.MinioUseSSL)
	default:
	}
	if err := r.Err(); err != nil {
		return config{}, err
	}
	return cfg, nil
}
