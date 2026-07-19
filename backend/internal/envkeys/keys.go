// Package envkeys centralizes the environment variable names the composition
// roots (cmd/server, cmd/worker) read, so the two binaries can't drift.
package envkeys

// Environment variable names read by the composition roots. The values are var
// *names*, not secrets; the //nolint:gosec directives suppress G101, whose
// entropy heuristic flags the longer "..._SECRET" strings (JWT_SECRET slips under).
const (
	DBURL               = "DATABASE_URL"
	JWTSecret           = "JWT_SECRET"
	JWTTTL              = "JWT_TTL"
	Port                = "PORT"
	MinioEndpoint       = "MINIO_ENDPOINT"
	MinioPublicEndpoint = "MINIO_PUBLIC_ENDPOINT"
	MinioRegion         = "MINIO_REGION"
	MinioAccessKey      = "MINIO_ACCESS_KEY"
	MinioSecretKey      = "MINIO_SECRET_KEY" //nolint:gosec // G101: env var name, not a credential
	MinioBucket         = "MINIO_BUCKET"
	MinioUseSSL         = "MINIO_USE_SSL"
	PharmacyABaseURL    = "PHARMACY_A_BASE_URL"
	PharmacyBBaseURL    = "PHARMACY_B_BASE_URL"
	TemporalHostPort    = "TEMPORAL_HOSTPORT"
	PharmacyASecret     = "PHARMACY_A_SECRET" //nolint:gosec // G101: env var name, not a credential
	PharmacyBSecret     = "PHARMACY_B_SECRET" //nolint:gosec // G101: env var name, not a credential
	OfferTTL            = "OFFER_TTL"
	ShippingBaseURL     = "SHIPPING_BASE_URL"
	WebhookBaseURL      = "WEBHOOK_BASE_URL"
	SMTPHost            = "SMTP_HOST"
	SMTPPort            = "SMTP_PORT"
	SMTPFrom            = "SMTP_FROM"
)
