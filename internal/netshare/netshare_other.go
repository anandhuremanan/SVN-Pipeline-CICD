//go:build !windows

package netshare

// Disconnect is only available on Windows.
func Disconnect(path string) {}

// Connect is only available on Windows.
func Connect(path, username, password string) error {
	return ErrUnsupported
}
