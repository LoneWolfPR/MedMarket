package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpapi "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// pngBytes is a minimal PNG header so http.DetectContentType reports image/png.
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

// stubRxService satisfies inbound.PrescriptionService; each test wires only the
// method it drives.
type stubRxService struct {
	uploadFn func(context.Context, inbound.PrescriptionInput) (inbound.PrescriptionView, error)
	getFn    func(context.Context, uuid.UUID, uuid.UUID) (inbound.PrescriptionView, error)
	listFn   func(context.Context, uuid.UUID) ([]inbound.PrescriptionView, error)
}

func (s stubRxService) Upload(
	ctx context.Context, in inbound.PrescriptionInput,
) (inbound.PrescriptionView, error) {
	return s.uploadFn(ctx, in)
}
func (s stubRxService) GetPrescription(
	ctx context.Context, userID, id uuid.UUID,
) (inbound.PrescriptionView, error) {
	return s.getFn(ctx, userID, id)
}
func (s stubRxService) GetPrescriptionList(
	ctx context.Context, userID uuid.UUID,
) ([]inbound.PrescriptionView, error) {
	return s.listFn(ctx, userID)
}

// newRxStack assembles the real HTTP stack with a configurable prescription
// service (a no-op user service satisfies the rest of NewAPI).
func newRxStack(t *testing.T, rxSvc inbound.PrescriptionService, ti outbound.TokenIssuer) http.Handler {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	h, err := httpapi.NewAuthHandler(httpapi.NewAuthHandlerParams{Logger: logger, Svc: stubService{}})
	require.NoError(t, err)
	ph, err := httpapi.NewPrescriptionHandler(httpapi.NewPrescriptionHandlerParams{Logger: logger, Svc: rxSvc})
	require.NoError(t, err)

	mux := http.NewServeMux()
	openapi.HandlerFromMux(httpapi.NewAPI(httpapi.NewAPIParams{
		Auth:         h,
		Prescription: ph,
		Logger:       logger,
		TokenIssuer:  ti,
	}), mux)
	return mux
}

// validRxFields returns the non-file form fields for a valid upload.
func validRxFields() map[string]string {
	return map[string]string{
		"physicianName": "Dr. Smith",
		"medName":       "atorvastatin",
		"strengthValue": "20",
		"strengthUnit":  "mg",
		"quantity":      "30",
	}
}

// multipartUpload encodes the given fields (and, if docData is non-nil, a
// "document" file part) into a multipart body and returns it with its
// Content-Type header.
func multipartUpload(t *testing.T, fields map[string]string, docData []byte) (*bytes.Buffer, string) {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	if docData != nil {
		fw, err := w.CreateFormFile("document", "rx.png")
		require.NoError(t, err)
		_, err = fw.Write(docData)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

// doUpload posts a multipart body to the upload route, optionally bearing a token.
func doUpload(
	t *testing.T, handler http.Handler, body *bytes.Buffer, contentType, token string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/prescriptions/upload", body)
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// buildRxView builds a PrescriptionView the service stub can return.
func buildRxView(t *testing.T, url string) inbound.PrescriptionView {
	t.Helper()

	ms, err := shared.NewMedStrength("20", "mg")
	require.NoError(t, err)
	rx, err := prescription.NewPrescription(prescription.NewPrescriptionParams{
		UserID:        uuid.New(),
		DocumentKey:   "prescriptions/x/y.png",
		PhysicianName: "Dr. Smith",
		MedName:       "atorvastatin",
		MedStrength:   ms,
		Qty:           30,
	})
	require.NoError(t, err)
	rx.ID = uuid.New()
	return inbound.PrescriptionView{Prescription: *rx, DocumentURL: url}
}

// tokenIssuer resolves any token to the given user ID, exercising the full
// middleware -> context -> handler path.
func tokenIssuer(userID uuid.UUID) stubIssuer {
	return stubIssuer{verifyFn: func(string) (uuid.UUID, error) { return userID, nil }}
}

// --- UploadPrescription -----------------------------------------------------

func TestUploadRoute_RequiresAuth(t *testing.T) {
	rxSvc := stubRxService{
		uploadFn: func(context.Context, inbound.PrescriptionInput) (inbound.PrescriptionView, error) {
			t.Fatal("service must not be reached without auth")
			return inbound.PrescriptionView{}, nil
		},
	}
	handler := newRxStack(t, rxSvc, stubIssuer{})

	body, ct := multipartUpload(t, validRxFields(), pngBytes)
	rec := doUpload(t, handler, body, ct, "") // no token

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUploadRoute_Success(t *testing.T) {
	userID := uuid.New()
	view := buildRxView(t, "https://signed/url")

	var got inbound.PrescriptionInput
	rxSvc := stubRxService{
		uploadFn: func(_ context.Context, in inbound.PrescriptionInput) (inbound.PrescriptionView, error) {
			got = in
			return view, nil
		},
	}
	handler := newRxStack(t, rxSvc, tokenIssuer(userID))

	body, ct := multipartUpload(t, validRxFields(), pngBytes)
	rec := doUpload(t, handler, body, ct, "any-token")

	require.Equal(t, http.StatusCreated, rec.Code)

	// The handler translated every form field, sniffed the content type, and
	// forwarded the authenticated user's ID and the document bytes.
	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, "Dr. Smith", got.PhysicianName)
	assert.Equal(t, "atorvastatin", got.MedName)
	assert.Equal(t, "20", got.MedStrengthValue)
	assert.Equal(t, "mg", got.MedStrengthUnit)
	assert.Equal(t, 30, got.Qty)
	assert.Equal(t, "image/png", got.ContentType)
	fileBytes, err := io.ReadAll(got.File)
	require.NoError(t, err)
	assert.Equal(t, pngBytes, fileBytes)

	var resp openapi.PrescriptionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, view.Prescription.ID, resp.Id)
	assert.Equal(t, "https://signed/url", resp.DocumentUrl)
}

func TestUploadRoute_MissingDocument(t *testing.T) {
	rxSvc := stubRxService{
		uploadFn: func(context.Context, inbound.PrescriptionInput) (inbound.PrescriptionView, error) {
			t.Fatal("service must not be called when the document is missing")
			return inbound.PrescriptionView{}, nil
		},
	}
	handler := newRxStack(t, rxSvc, tokenIssuer(uuid.New()))

	body, ct := multipartUpload(t, validRxFields(), nil) // no document part
	rec := doUpload(t, handler, body, ct, "any-token")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, inbound.ErrInvalidFile.Error(), decodeMessage(t, rec))
}

func TestUploadRoute_FileTooLarge(t *testing.T) {
	rxSvc := stubRxService{
		uploadFn: func(context.Context, inbound.PrescriptionInput) (inbound.PrescriptionView, error) {
			t.Fatal("service must not be called when the document is too large")
			return inbound.PrescriptionView{}, nil
		},
	}
	handler := newRxStack(t, rxSvc, tokenIssuer(uuid.New()))

	// One byte past the handler's 10 MiB cap.
	oversize := bytes.Repeat([]byte("a"), (10<<20)+1)
	body, ct := multipartUpload(t, validRxFields(), oversize)
	rec := doUpload(t, handler, body, ct, "any-token")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, inbound.ErrFileTooLarge.Error(), decodeMessage(t, rec))
}

func TestUploadRoute_NonIntegerQuantity(t *testing.T) {
	rxSvc := stubRxService{
		uploadFn: func(context.Context, inbound.PrescriptionInput) (inbound.PrescriptionView, error) {
			t.Fatal("service must not be called when quantity is not an integer")
			return inbound.PrescriptionView{}, nil
		},
	}
	handler := newRxStack(t, rxSvc, tokenIssuer(uuid.New()))

	fields := validRxFields()
	fields["quantity"] = "not-a-number"
	body, ct := multipartUpload(t, fields, pngBytes)
	rec := doUpload(t, handler, body, ct, "any-token")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUploadRoute_ServiceErrorsMapToStatus(t *testing.T) {
	tests := map[string]struct {
		svcErr   error
		wantCode int
	}{
		"invalid file -> 400": {svcErr: inbound.ErrInvalidFile, wantCode: http.StatusBadRequest},
		"validation -> 400":   {svcErr: inbound.ErrValidation, wantCode: http.StatusBadRequest},
		"unexpected -> 500":   {svcErr: errBoom, wantCode: http.StatusInternalServerError},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rxSvc := stubRxService{
				uploadFn: func(context.Context, inbound.PrescriptionInput) (inbound.PrescriptionView, error) {
					return inbound.PrescriptionView{}, tc.svcErr
				},
			}
			handler := newRxStack(t, rxSvc, tokenIssuer(uuid.New()))

			body, ct := multipartUpload(t, validRxFields(), pngBytes)
			rec := doUpload(t, handler, body, ct, "any-token")

			require.Equal(t, tc.wantCode, rec.Code)
			if tc.wantCode == http.StatusInternalServerError {
				assert.NotContains(t, rec.Body.String(), "boom", "internal error text must not leak")
			}
		})
	}
}

// --- ListPrescriptions ------------------------------------------------------

func TestListRoute_RequiresAuth(t *testing.T) {
	rxSvc := stubRxService{
		listFn: func(context.Context, uuid.UUID) ([]inbound.PrescriptionView, error) {
			t.Fatal("service must not be reached without auth")
			return nil, nil
		},
	}
	handler := newRxStack(t, rxSvc, stubIssuer{})

	rec := do(t, handler, http.MethodGet, "/api/prescriptions", nil, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListRoute_Success(t *testing.T) {
	userID := uuid.New()
	views := []inbound.PrescriptionView{
		buildRxView(t, "https://signed/one"),
		buildRxView(t, "https://signed/two"),
	}
	var askedID uuid.UUID
	rxSvc := stubRxService{
		listFn: func(_ context.Context, uid uuid.UUID) ([]inbound.PrescriptionView, error) {
			askedID = uid
			return views, nil
		},
	}
	handler := newRxStack(t, rxSvc, tokenIssuer(userID))

	rec := do(t, handler, http.MethodGet, "/api/prescriptions", nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID, askedID, "the authenticated user's ID should reach the service")

	var resp []openapi.PrescriptionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, "https://signed/one", resp[0].DocumentUrl)
	assert.Equal(t, "https://signed/two", resp[1].DocumentUrl)
}

func TestListRoute_ServiceErrorYields500(t *testing.T) {
	rxSvc := stubRxService{
		listFn: func(context.Context, uuid.UUID) ([]inbound.PrescriptionView, error) {
			return nil, errBoom
		},
	}
	handler := newRxStack(t, rxSvc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodGet, "/api/prescriptions", nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "boom", "internal error text must not leak")
}

// decodeMessage extracts the "message" field from a JSON error response body.
func decodeMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var e openapi.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	return e.Message
}
