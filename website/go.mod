// This file is not a Go project. It marks website/ as outside the Tread Go
// module, so `go build/vet/test ./...` in the repository root never descends
// into website/node_modules (some npm packages ship .go files).
module tread-website-not-go

go 1.25.0
