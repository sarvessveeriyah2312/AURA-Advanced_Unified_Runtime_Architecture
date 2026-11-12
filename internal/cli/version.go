package cli

// Version is the AURA compiler version reported by `aura version`.
//
// It is a variable only so release builds can override it at link time:
//
//	go build -ldflags "-X github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/cli.Version=0.1.0" ./cmd/aura
//
// It must not be modified at run time.
var Version = "0.1.0"
