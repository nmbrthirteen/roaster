//go:build !windows

package secret

// No platform keystore here. The file permissions are the only protection,
// which is why the token is scoped and revocable rather than trusted.
func protect(plain []byte) ([]byte, error)    { return plain, nil }
func unprotect(sealed []byte) ([]byte, error) { return sealed, nil }
