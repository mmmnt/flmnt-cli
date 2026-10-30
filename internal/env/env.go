// Package env is the one place the CLI reads its own environment variables. Each is named FLMNT_<NAME>;
// the name it had before the product was flmnt, QUORUM_<NAME>, is still read when the FLMNT_ name is
// unset, so a script written against the former name keeps working.
package env

import "os"

// Get returns FLMNT_<name>, else QUORUM_<name>, else "".
func Get(name string) string {
	if v := os.Getenv("FLMNT_" + name); v != "" {
		return v
	}
	return os.Getenv("QUORUM_" + name)
}
