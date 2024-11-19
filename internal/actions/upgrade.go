package actions

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/utils/colors"
)

type InstallationMethod int

const (
	InstallationMethodUnknown InstallationMethod = iota
	InstallationMethodHomebrew
	InstallationMethodAUR
	InstallationMethodDeb
	InstallationMethodRPM
	InstallationMethodBinary
)

// UpgradeCLI upgrades the av CLI to the latest version.
func UpgradeCLI(osName, arch string) error {
	installedBy, err := DetectInstallationMethod()
	if err != nil {
		return fmt.Errorf("failed to detect installation method: %w", err)
	}

	switch installedBy {
	case InstallationMethodHomebrew:
		fmt.Println(colors.CliCmd("Upgrading via Homebrew..."))
		return upgradeHomebrew()
	case InstallationMethodAUR:
		fmt.Println(colors.CliCmd("Upgrading via AUR..."))
		return upgradeAUR()
	case InstallationMethodDeb:
		fmt.Println(colors.CliCmd("Upgrading via apt..."))
		return upgradeDeb()
	case InstallationMethodRPM:
		fmt.Println(colors.CliCmd("Upgrading via yum..."))
		return upgradeRPM()
	case InstallationMethodBinary:
		fmt.Println(colors.CliCmd("Upgrading binary installation..."))
		return upgradeBinary(osName, arch)
	default:
		return fmt.Errorf("unknown installation method")
	}
}

func upgradeHomebrew() error {
	cmd := exec.Command("brew", "upgrade", "av")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func upgradeAUR() error {
	// Try common AUR helpers
	aurHelpers := []string{"yay", "paru", "pamac"}
	for _, helper := range aurHelpers {
		if path, err := exec.LookPath(helper); err == nil {
			cmd := exec.Command(path, "-S", "av-cli-bin")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}
	return fmt.Errorf("no supported AUR helper found (tried: yay, paru, pamac)")
}

func upgradeDeb() error {
	latestVersion, err := config.FetchLatestVersion()
	if err != nil {
		return err
	}

	// Download the .deb file
	url := fmt.Sprintf("https://github.com/aviator-co/av/releases/download/%s/av_%s_linux_%s.deb",
		latestVersion, latestVersion, runtime.GOARCH)

	fmt.Printf(colors.CliCmd("Downloading %s...\n"), url)

	tmpFile, err := downloadFile(url)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// Install the .deb file
	cmd := exec.Command("sudo", "dpkg", "-i", tmpFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func upgradeRPM() error {
	latestVersion, err := config.FetchLatestVersion()
	if err != nil {
		return err
	}

	// Download the .rpm file
	url := fmt.Sprintf("https://github.com/aviator-co/av/releases/download/%s/av_%s_linux_%s.rpm",
		latestVersion, latestVersion, runtime.GOARCH)

	fmt.Printf(colors.CliCmd("Downloading %s...\n"), url)

	tmpFile, err := downloadFile(url)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// Install the .rpm file
	cmd := exec.Command("sudo", "rpm", "-U", tmpFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// downloadFile downloads a file and returns the path to the temporary file
func downloadFile(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "av-upgrade-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// DetectInstallationMethod determines how the av CLI was installed.
func DetectInstallationMethod() (InstallationMethod, error) {
	executable, err := os.Executable()
	if err != nil {
		return InstallationMethodUnknown, fmt.Errorf("failed to determine current executable: %w", err)
	}

	if strings.Contains(executable, "/Cellar/") || strings.Contains(executable, "/Homebrew/") {
		return InstallationMethodHomebrew, nil
	}

	if strings.Contains(executable, "/.cache/yay/") || strings.Contains(executable, "/pkg/") {
		return InstallationMethodAUR, nil
	}

	// Check for .deb package
	if _, err := exec.LookPath("dpkg"); err == nil {
		cmd := exec.Command("dpkg", "-S", executable)
		if err := cmd.Run(); err == nil {
			return InstallationMethodDeb, nil
		}
	}

	// Check for RPM package
	if _, err := exec.LookPath("rpm"); err == nil {
		cmd := exec.Command("rpm", "-qf", executable)
		if err := cmd.Run(); err == nil {
			return InstallationMethodRPM, nil
		}
	}

	return InstallationMethodBinary, nil
}

// upgradeBinary handles the upgrade process for binary installations.
func upgradeBinary(osName, arch string) error {
	latestVersion, err := config.FetchLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to fetch latest version: %w", err)
	}

	if config.Version == config.VersionDev {
		return errors.New("cannot upgrade development version")
	}

	if config.Version == latestVersion {
		fmt.Println(colors.Success("You are already using the latest version."))
		return nil
	}

	downloadURL, err := getDownloadURL(latestVersion, osName, arch)
	if err != nil {
		return err
	}

	fmt.Printf(colors.CliCmd("Downloading %s...\n"), downloadURL)

	tmpFile, err := downloadFile(downloadURL)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find current executable: %w", err)
	}

	fmt.Println(colors.CliCmd("Extracting and installing..."))

	// Create a backup of the current executable
	backupPath := executable + ".backup"
	if err := os.Rename(executable, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Extract and install the new version
	if err := extractAndInstall(tmpFile, osName, executable); err != nil {
		// Restore backup on failure
		if restoreErr := os.Rename(backupPath, executable); restoreErr != nil {
			return fmt.Errorf("failed to restore backup after failed upgrade: %w", restoreErr)
		}
		return err
	}

	// Remove backup on success
	os.Remove(backupPath)

	fmt.Printf(colors.Success("Successfully upgraded to version %s\n"), latestVersion)
	return nil
}

func extractAndInstall(archivePath, osName, targetPath string) error {
	switch osName {
	case "darwin", "linux":
		return extractTarGzAndInstall(archivePath, targetPath)
	case "windows":
		return extractZipAndInstall(archivePath, targetPath)
	default:
		return fmt.Errorf("unsupported OS: %s", osName)
	}
}

func extractTarGzAndInstall(archivePath, targetPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if strings.HasSuffix(header.Name, "av") || strings.HasSuffix(header.Name, "av.exe") {
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("executable not found in archive")
}

func extractZipAndInstall(archivePath, targetPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "av.exe") {
			rc, err := file.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("av.exe not found in archive")
}

func getDownloadURL(version, osName, arch string) (string, error) {
	var fileExt, osArch string
	switch osName {
	case "darwin":
		fileExt = "tar.gz"
		osArch = "darwin"
	case "linux":
		fileExt = "tar.gz"
		osArch = "linux"
	case "windows":
		fileExt = "zip"
		osArch = "windows"
	default:
		return "", fmt.Errorf("unsupported OS: %s", osName)
	}

	versionWithoutV := strings.TrimPrefix(version, "v")

	return fmt.Sprintf("https://github.com/aviator-co/av/releases/download/%s/av_%s_%s_%s.%s",
		version, versionWithoutV, osArch, arch, fileExt), nil
}
