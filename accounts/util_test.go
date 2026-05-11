package accounts

import "os"

// osStat is a tiny indirection so the permission-check test does not have
// to import os in the main test file. Keeps test boilerplate minimal.
func osStat(path string) (os.FileInfo, error) { return os.Stat(path) }
