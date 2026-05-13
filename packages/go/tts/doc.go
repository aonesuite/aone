// Package tts provides the Go SDK for Aone text-to-speech APIs.
//
// A Client can list available voices and synthesize text into audio using the
// shared Aone API endpoint. The SDK reads AONE_API_KEY when Config.APIKey is
// empty, and the shared Aone endpoint environment variable when Config.Endpoint
// is empty.
package tts
