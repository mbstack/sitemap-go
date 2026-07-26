// Package config defines the runtime configuration for a Scanner.
// Config is the only thing library callers need to construct; the
// rest of the library reads its tunables from this struct.
//
// All fields are zero-initialisable except DataDir and Logger; use
// DefaultConfig as a starting point, fill the required fields, and
// pass the result to NewScanner.
package config
