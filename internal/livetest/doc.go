// Package livetest holds tests that talk to a real Fragment API.
//
// Everything else in this repo stops at the request boundary: the conformance
// tests assert on the JSON a batch would send, using a client that encodes and
// returns rather than transmits. That leaves one thing unverified, and the shared
// specification calls it out as unverified in every SDK — whether the API accepts
// an entry object with a type and no lines, which is the shape a typed payload
// necessarily produces.
//
// These tests answer that. They are skipped unless credentials are present, so
// `go test ./...` stays offline by default:
//
//	FRAGMENT_CLIENT_ID     client ID from the dashboard
//	FRAGMENT_CLIENT_SECRET client secret
//	FRAGMENT_SCOPE         OAuth scope
//	FRAGMENT_AUTH_URL      OAuth URL
//	FRAGMENT_API_URL       API URL
//
// They write to whatever Workspace those credentials point at. Each run stores a
// new version of one fixed Schema and creates a fresh Ledger, so runs do not
// collide with each other, but they do accumulate Ledgers. Point them at a
// scratch Workspace, not production.
package livetest
