package outbound

// PasswordHasher is the port used for logic around hashing and comparing passwords
type PasswordHasher interface {
	Hash(pw string) (string, error)
	Compare(hash, plain string) error
}
