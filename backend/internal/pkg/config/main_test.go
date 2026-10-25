package config

import (
	"os"
	"testing"

	"github.com/tiredbooy/Rum/backend/internal/pkg/testenv"
)

// TestMain keeps every test off the real user config dir.
func TestMain(m *testing.M) { os.Exit(testenv.Run(m)) }
