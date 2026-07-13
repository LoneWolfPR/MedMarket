package app_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/app"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// newRxService constructs a PrescriptionService with the given fakes and a
// discard logger.
func newRxService(
	t *testing.T,
	repo outbound.PrescriptionRepository,
	fs outbound.FileStorage,
) *app.PrescriptionService {
	t.Helper()

	svc, err := app.NewPrescriptionService(app.NewPrescriptionServiceParams{
		Logger:           slog.New(slog.DiscardHandler),
		PrescriptionRepo: repo,
		FileStorage:      fs,
	})
	require.NoError(t, err)
	return svc
}

// validUploadInput returns a PrescriptionInput that passes all validation.
func validUploadInput(userID uuid.UUID) inbound.PrescriptionInput {
	return inbound.PrescriptionInput{
		UserID:           userID,
		PhysicianName:    "Dr. Smith",
		MedName:          "atorvastatin",
		MedStrengthValue: "20",
		MedStrengthUnit:  "mg",
		Qty:              30,
		ContentType:      "application/pdf",
		File:             strings.NewReader("file-bytes"),
	}
}

// echoCreate is a repo fake that returns the prescription it was given with a
// freshly assigned ID, imitating a successful insert.
func echoCreate(_ context.Context, p *prescription.Prescription) (*prescription.Prescription, error) {
	p.ID = uuid.New()
	return p, nil
}

// buildRx builds a persisted-looking domain prescription owned by userID.
func buildRx(t *testing.T, userID uuid.UUID) prescription.Prescription {
	t.Helper()

	ms, err := shared.NewMedStrength("20", "mg")
	require.NoError(t, err)
	rx, err := prescription.NewPrescription(prescription.NewPrescriptionParams{
		UserID:        userID,
		DocumentKey:   "prescriptions/x/y.pdf",
		PhysicianName: "Dr. Smith",
		MedName:       "atorvastatin",
		MedStrength:   ms,
		Qty:           30,
	})
	require.NoError(t, err)
	rx.ID = uuid.New()
	return *rx
}

// --- NewPrescriptionService -------------------------------------------------

func TestNewPrescriptionService_Validation(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	repo := fakePrescriptionRepo{}
	fs := fakeFileStorage{}

	tests := map[string]app.NewPrescriptionServiceParams{
		"missing logger":       {PrescriptionRepo: repo, FileStorage: fs},
		"missing repository":   {Logger: logger, FileStorage: fs},
		"missing file storage": {Logger: logger, PrescriptionRepo: repo},
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			svc, err := app.NewPrescriptionService(params)
			require.Error(t, err)
			assert.Nil(t, svc)
		})
	}
}

// --- Upload -----------------------------------------------------------------

func TestUpload_InvalidContentType(t *testing.T) {
	repo := fakePrescriptionRepo{
		createFn: func(context.Context, *prescription.Prescription) (*prescription.Prescription, error) {
			t.Fatal("Create must not be called for an invalid content type")
			return nil, nil
		},
	}
	fs := fakeFileStorage{
		putFn: func(context.Context, string, string, io.Reader) error {
			t.Fatal("Put must not be called for an invalid content type")
			return nil
		},
	}
	svc := newRxService(t, repo, fs)

	in := validUploadInput(uuid.New())
	in.ContentType = "text/plain"

	_, err := svc.Upload(context.Background(), in)
	require.ErrorIs(t, err, inbound.ErrInvalidFile)
}

func TestUpload_InvalidMetadataValidatesBeforePut(t *testing.T) {
	// Bad strength unit and bad quantity both fail domain validation, which the
	// service must do BEFORE writing anything to storage (no orphaned objects).
	tests := map[string]func(*inbound.PrescriptionInput){
		"bad strength unit": func(in *inbound.PrescriptionInput) { in.MedStrengthUnit = "invalid" },
		"non-positive qty":  func(in *inbound.PrescriptionInput) { in.Qty = 0 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			repo := fakePrescriptionRepo{
				createFn: func(context.Context, *prescription.Prescription) (*prescription.Prescription, error) {
					t.Fatal("Create must not be called when metadata is invalid")
					return nil, nil
				},
			}
			fs := fakeFileStorage{
				putFn: func(context.Context, string, string, io.Reader) error {
					t.Fatal("Put must not be called when metadata is invalid")
					return nil
				},
			}
			svc := newRxService(t, repo, fs)

			in := validUploadInput(uuid.New())
			mutate(&in)

			_, err := svc.Upload(context.Background(), in)
			require.ErrorIs(t, err, inbound.ErrValidation)
		})
	}
}

func TestUpload_PutFailsStopsBeforeCreate(t *testing.T) {
	repo := fakePrescriptionRepo{
		createFn: func(context.Context, *prescription.Prescription) (*prescription.Prescription, error) {
			t.Fatal("Create must not be called when Put fails")
			return nil, nil
		},
	}
	fs := fakeFileStorage{
		putFn: func(context.Context, string, string, io.Reader) error { return errBoom },
	}
	svc := newRxService(t, repo, fs)

	_, err := svc.Upload(context.Background(), validUploadInput(uuid.New()))
	require.ErrorIs(t, err, errBoom)
	// A storage failure is infrastructure, not a client validation error.
	assert.NotErrorIs(t, err, inbound.ErrValidation)
	assert.NotErrorIs(t, err, inbound.ErrInvalidFile)
}

func TestUpload_CreateFailsSurfacesGenericError(t *testing.T) {
	repo := fakePrescriptionRepo{
		createFn: func(context.Context, *prescription.Prescription) (*prescription.Prescription, error) {
			return nil, errBoom
		},
	}
	fs := fakeFileStorage{
		putFn: func(context.Context, string, string, io.Reader) error { return nil },
	}
	svc := newRxService(t, repo, fs)

	_, err := svc.Upload(context.Background(), validUploadInput(uuid.New()))
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, inbound.ErrValidation)
}

func TestUpload_Success(t *testing.T) {
	userID := uuid.New()

	var gotKey, gotContentType string
	var gotFileBytes []byte
	fs := fakeFileStorage{
		putFn: func(_ context.Context, key, contentType string, r io.Reader) error {
			gotKey = key
			gotContentType = contentType
			b, err := io.ReadAll(r)
			require.NoError(t, err)
			gotFileBytes = b
			return nil
		},
		presignFn: func(_ context.Context, key string) (string, error) {
			assert.Equal(t, gotKey, key, "presign should use the same key that was stored")
			return "https://signed/url", nil
		},
	}
	repo := fakePrescriptionRepo{createFn: echoCreate}
	svc := newRxService(t, repo, fs)

	view, err := svc.Upload(context.Background(), validUploadInput(userID))
	require.NoError(t, err)

	// The object key is namespaced per user and carries the content-type's ext.
	assert.True(t, strings.HasPrefix(gotKey, "prescriptions/"+userID.String()+"/"),
		"key should be namespaced under the user: %s", gotKey)
	assert.True(t, strings.HasSuffix(gotKey, ".pdf"), "key should carry the pdf extension: %s", gotKey)
	assert.Equal(t, "application/pdf", gotContentType)
	assert.Equal(t, []byte("file-bytes"), gotFileBytes)

	assert.Equal(t, "https://signed/url", view.DocumentURL)
	assert.Equal(t, gotKey, view.Prescription.DocumentKey)
	assert.Equal(t, userID, view.Prescription.UserID)
	assert.Equal(t, "Dr. Smith", view.Prescription.PhysicianName)
	assert.Equal(t, "atorvastatin", view.Prescription.MedName)
	assert.Equal(t, "20", view.Prescription.MedStrength.Value())
	assert.Equal(t, "mg", view.Prescription.MedStrength.Unit())
	assert.Equal(t, 30, view.Prescription.Qty)
	assert.NotEqual(t, uuid.Nil, view.Prescription.ID)
}

func TestUpload_ContentTypeToExtension(t *testing.T) {
	cases := map[string]string{
		"application/pdf": ".pdf",
		"image/png":       ".png",
		"image/jpeg":      ".jpg",
	}
	for contentType, ext := range cases {
		t.Run(contentType, func(t *testing.T) {
			var gotKey string
			fs := fakeFileStorage{
				putFn: func(_ context.Context, key, _ string, _ io.Reader) error {
					gotKey = key
					return nil
				},
				presignFn: func(context.Context, string) (string, error) { return "u", nil },
			}
			svc := newRxService(t, fakePrescriptionRepo{createFn: echoCreate}, fs)

			in := validUploadInput(uuid.New())
			in.ContentType = contentType

			_, err := svc.Upload(context.Background(), in)
			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(gotKey, ext), "key %q should end with %q", gotKey, ext)
		})
	}
}

func TestUpload_PresignFailureStillReturnsRecord(t *testing.T) {
	// A presign failure is logged and swallowed: the record was created, so the
	// upload succeeds with an empty URL rather than failing the whole request.
	fs := fakeFileStorage{
		putFn:     func(context.Context, string, string, io.Reader) error { return nil },
		presignFn: func(context.Context, string) (string, error) { return "", errBoom },
	}
	svc := newRxService(t, fakePrescriptionRepo{createFn: echoCreate}, fs)

	view, err := svc.Upload(context.Background(), validUploadInput(uuid.New()))
	require.NoError(t, err)
	assert.Empty(t, view.DocumentURL)
	assert.NotEqual(t, uuid.Nil, view.Prescription.ID)
}

// --- GetPrescription --------------------------------------------------------

func TestGetPrescription_NotFound(t *testing.T) {
	repo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return nil, outbound.ErrPrescriptionNotFound
		},
	}
	fs := fakeFileStorage{
		presignFn: func(context.Context, string) (string, error) {
			t.Fatal("presign must not be called when the record is not found")
			return "", nil
		},
	}
	svc := newRxService(t, repo, fs)

	_, err := svc.GetPrescription(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, inbound.ErrPrescriptionNotFound)
}

func TestGetPrescription_RepoErrorSurfacesGeneric(t *testing.T) {
	repo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return nil, errBoom
		},
	}
	svc := newRxService(t, repo, fakeFileStorage{})

	_, err := svc.GetPrescription(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, inbound.ErrPrescriptionNotFound)
}

func TestGetPrescription_OwnershipMismatchHidesRecord(t *testing.T) {
	owner := uuid.New()
	rx := buildRx(t, owner)
	repo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	fs := fakeFileStorage{
		presignFn: func(context.Context, string) (string, error) {
			t.Fatal("presign must not be called for a record the caller does not own")
			return "", nil
		},
	}
	svc := newRxService(t, repo, fs)

	// A different user asking for the record gets the same not-found response as
	// a missing record — existence is not leaked.
	_, err := svc.GetPrescription(context.Background(), uuid.New(), rx.ID)
	require.ErrorIs(t, err, inbound.ErrPrescriptionNotFound)
}

func TestGetPrescription_OwnerSucceeds(t *testing.T) {
	owner := uuid.New()
	rx := buildRx(t, owner)
	repo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	fs := fakeFileStorage{
		presignFn: func(_ context.Context, key string) (string, error) {
			assert.Equal(t, rx.DocumentKey, key)
			return "https://signed/url", nil
		},
	}
	svc := newRxService(t, repo, fs)

	view, err := svc.GetPrescription(context.Background(), owner, rx.ID)
	require.NoError(t, err)
	assert.Equal(t, rx.ID, view.Prescription.ID)
	assert.Equal(t, "https://signed/url", view.DocumentURL)
}

func TestGetPrescription_PresignFailureStillReturnsRecord(t *testing.T) {
	owner := uuid.New()
	rx := buildRx(t, owner)
	repo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	fs := fakeFileStorage{
		presignFn: func(context.Context, string) (string, error) { return "", errBoom },
	}
	svc := newRxService(t, repo, fs)

	view, err := svc.GetPrescription(context.Background(), owner, rx.ID)
	require.NoError(t, err)
	assert.Equal(t, rx.ID, view.Prescription.ID)
	assert.Empty(t, view.DocumentURL)
}

// --- GetPrescriptionList ----------------------------------------------------

func TestGetPrescriptionList_RepoError(t *testing.T) {
	repo := fakePrescriptionRepo{
		listFn: func(context.Context, uuid.UUID) ([]prescription.Prescription, error) {
			return nil, errBoom
		},
	}
	svc := newRxService(t, repo, fakeFileStorage{})

	_, err := svc.GetPrescriptionList(context.Background(), uuid.New())
	require.ErrorIs(t, err, errBoom)
}

func TestGetPrescriptionList_Success(t *testing.T) {
	owner := uuid.New()
	list := []prescription.Prescription{buildRx(t, owner), buildRx(t, owner)}
	repo := fakePrescriptionRepo{
		listFn: func(_ context.Context, userID uuid.UUID) ([]prescription.Prescription, error) {
			assert.Equal(t, owner, userID)
			return list, nil
		},
	}
	fs := fakeFileStorage{
		presignFn: func(context.Context, string) (string, error) { return "https://signed/url", nil },
	}
	svc := newRxService(t, repo, fs)

	views, err := svc.GetPrescriptionList(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, views, 2)
	for _, v := range views {
		assert.Equal(t, "https://signed/url", v.DocumentURL)
	}
}

func TestGetPrescriptionList_PresignFailurePerRowIsSwallowed(t *testing.T) {
	// A presign failure on one row logs and continues; the row is still returned
	// with an empty URL, and the list call succeeds.
	owner := uuid.New()
	list := []prescription.Prescription{buildRx(t, owner)}
	repo := fakePrescriptionRepo{
		listFn: func(context.Context, uuid.UUID) ([]prescription.Prescription, error) {
			return list, nil
		},
	}
	fs := fakeFileStorage{
		presignFn: func(context.Context, string) (string, error) { return "", errBoom },
	}
	svc := newRxService(t, repo, fs)

	views, err := svc.GetPrescriptionList(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Empty(t, views[0].DocumentURL)
}
