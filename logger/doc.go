// Package logger provides a configured zerolog.Logger for the
// sitemap-go library and CLI. The default logger writes structured
// JSON to stderr; callers may construct a custom logger (e.g. with
// ConsoleWriter) and pass it via config.Config.Logger.
package logger
