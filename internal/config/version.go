package config

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

const VersionDev = "<dev>"

// Version is the version of the av application.
// It is set automatically when creating release builds.
var Version = VersionDev

func FetchLatestVersion() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := home + "/.cache/av"
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return "", err
	}
	cacheFile := cacheDir + "/version-check"
	stat, _ := os.Stat(cacheFile)

	if stat != nil && time.Since(stat.ModTime()) <= (24*time.Hour) {
		data, err := os.ReadFile(cacheFile)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://api.github.com/repos/aviator-co/av/releases/latest",
		nil,
	)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var data struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if err := os.WriteFile(cacheFile, []byte(data.Name), os.ModePerm); err != nil {
		return "", err
	}

	return data.Name, nil
}
