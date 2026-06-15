package server

import "github.com/tiredbooy/Rum/backend/internal/pkg/config"

type PublicSettings struct {
	CloseToTray   bool
	ConfirmOnExit bool
	// Add any other fields you need from config.Setting
}

// LoadSettings reads the internal config and returns a public view.
func LoadSettings() (PublicSettings, error) {
	var s config.Setting
	if err := s.LoadSettingMetadata(); err != nil {
		return PublicSettings{}, err
	}

	return PublicSettings{
		
		ConfirmOnExit: s.ConfirmOnExit,
	}, nil
}
