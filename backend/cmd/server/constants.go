package main

const (
	EnvDBURL               = "DATABASE_URL"
	EnvJWTSecret           = "JWT_SECRET"
	EnvJWTTTL              = "JWT_TTL"
	EnvPort                = "PORT"
	EnvMinioEndpoint       = "MINIO_ENDPOINT"
	EnvMinioPublicEndpoint = "MINIO_PUBLIC_ENDPOINT"
	EnvMinioRegion         = "MINIO_REGION"
	EnvMinioAccessKey      = "MINIO_ACCESS_KEY"
	EnvMinioSecretKey      = "MINIO_SECRET_KEY" //nolint:gosec // G101: env var name, not a credential
	EnvMinioBucket         = "MINIO_BUCKET"
	EnvMinioUseSSL         = "MINIO_USE_SSL"
	EnvPharmacyABaseURL    = "PHARMACY_A_BASE_URL"
	EnvPharmacyBBaseURL    = "PHARMACY_B_BASE_URL"
	// These are env var *names*, not secret values; gosec G101 flags the longer
	// "..._SECRET" strings on its entropy heuristic (JWT_SECRET slips under it).
	EnvPharmacyASecret = "PHARMACY_A_SECRET" //nolint:gosec // G101: env var name, not a credential
	EnvPharmacyBSecret = "PHARMACY_B_SECRET" //nolint:gosec // G101: env var name, not a credential
)
