package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// UserState is per-user state that is saved to their XDG_STATE_HOME directory.
var UserState struct {
	NotifiedStackSyncChange bool
}

// LoadUserState loads the user state.
func LoadUserState() error {
	pth, err := xdg.SearchStateFile(filepath.Join("av", "user-state.json"))
	if err != nil {
		// If the file doesn't exist, that's fine.
		return nil
	}
	bs, err := os.ReadFile(pth)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bs, &UserState); err != nil {
		return err
	}
	return nil
}

// SaveUserState saves the user state.
func SaveUserState() error {
	bs, err := json.Marshal(UserState)
	if err != nil {
		return err
	}
	pth, err := xdg.StateFile(filepath.Join("av", "user-state.json"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(pth, bs, 0644); err != nil {
		return err
	}
	return nil
}
