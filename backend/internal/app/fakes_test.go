package app_test

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
)

// Hand-rolled fakes for the outbound ports. Each method delegates to a function
// field so a test sets only the behavior it exercises; an unset field panics,
// which surfaces an unexpected call as a test failure.

type fakeUserRepo struct {
	createFn     func(ctx context.Context, u *user.User) (*user.User, error)
	getByIDFn    func(ctx context.Context, id uuid.UUID) (*user.User, error)
	getByEmailFn func(ctx context.Context, email shared.Email) (*user.User, error)
}

func (f fakeUserRepo) Create(ctx context.Context, u *user.User) (*user.User, error) {
	return f.createFn(ctx, u)
}

func (f fakeUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	return f.getByIDFn(ctx, id)
}

func (f fakeUserRepo) GetByEmail(ctx context.Context, email shared.Email) (*user.User, error) {
	return f.getByEmailFn(ctx, email)
}

type fakePasswordHasher struct {
	hashFn    func(pw string) (string, error)
	compareFn func(hash, plain string) error
}

func (f fakePasswordHasher) Hash(pw string) (string, error) { return f.hashFn(pw) }
func (f fakePasswordHasher) Compare(hash, plain string) error {
	return f.compareFn(hash, plain)
}

type fakeTokenIssuer struct {
	issueFn  func(userID uuid.UUID) (string, error)
	verifyFn func(token string) (uuid.UUID, error)
}

func (f fakeTokenIssuer) Issue(userID uuid.UUID) (string, error) { return f.issueFn(userID) }
func (f fakeTokenIssuer) Verify(token string) (uuid.UUID, error) { return f.verifyFn(token) }

type fakePrescriptionRepo struct {
	createFn  func(ctx context.Context, p *prescription.Prescription) (*prescription.Prescription, error)
	getByIDFn func(ctx context.Context, id uuid.UUID) (*prescription.Prescription, error)
	listFn    func(ctx context.Context, userID uuid.UUID) ([]prescription.Prescription, error)
}

func (f fakePrescriptionRepo) Create(
	ctx context.Context, p *prescription.Prescription,
) (*prescription.Prescription, error) {
	return f.createFn(ctx, p)
}

func (f fakePrescriptionRepo) GetByID(
	ctx context.Context, id uuid.UUID,
) (*prescription.Prescription, error) {
	return f.getByIDFn(ctx, id)
}

func (f fakePrescriptionRepo) List(
	ctx context.Context, userID uuid.UUID,
) ([]prescription.Prescription, error) {
	return f.listFn(ctx, userID)
}

type fakeFileStorage struct {
	putFn     func(ctx context.Context, key, contentType string, reader io.Reader) error
	presignFn func(ctx context.Context, key string) (string, error)
}

func (f fakeFileStorage) Put(ctx context.Context, key, contentType string, reader io.Reader) error {
	return f.putFn(ctx, key, contentType, reader)
}

func (f fakeFileStorage) GetPresignedURL(ctx context.Context, key string) (string, error) {
	return f.presignFn(ctx, key)
}
