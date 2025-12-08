package bitwarden

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".cache"), nil
}

func writeBWSession(token string) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return err
	}

	sessionFile := filepath.Join(cacheDir, ".bw_session")

	content := fmt.Sprintf("BW_SESSION=%s\n", token)
	return os.WriteFile(sessionFile, []byte(content), 0600)
}

func readBWSession() (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}

	sessionFile := filepath.Join(cacheDir, ".bw_session")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	content := strings.TrimSpace(string(data))
	prefix := "BW_SESSION="
	if strings.HasPrefix(content, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(content, prefix)), nil
	}

	return content, nil
}

func unlockRaw() (string, error) {
	unlockCmd := exec.Command("bw", "unlock", "--raw")
	unlockCmd.Stdin = os.Stdin
	unlockCmd.Stderr = os.Stderr
	sessionToken, err := unlockCmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(sessionToken)), nil
}

func GetSessionOrLogin() (string, error) {
	cachedToken, _ := readBWSession()

	statusCmd := exec.Command("bw", "status")
	if cachedToken != "" {
		statusCmd.Env = append(os.Environ(), "BW_SESSION="+cachedToken)
	}

	output, err := statusCmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	var data map[string]any
	err = json.Unmarshal(output, &data)
	if err != nil {
		return "", fmt.Errorf("error Unmarshalling JSON: %w", err)
	}
	if status, ok := data["status"].(string); ok {
		fmt.Println("Status: ", status)

		var sessionToken string

		switch status {
		case "unauthenticated":
			loginCmd := exec.Command("bw", "login")
			loginCmd.Stdin = os.Stdin
			loginCmd.Stdout = os.Stdout
			loginCmd.Stderr = os.Stderr
			err := loginCmd.Run()
			if err != nil {
				return "", err
			}
			sessionToken, err = unlockRaw()
			if err != nil {
				return "", fmt.Errorf("error unmarshalling JSON: %w", err)
			}
			err = writeBWSession(sessionToken)
			if err != nil {
				return "", fmt.Errorf("unlock after login failed: %w", err)
			}
		case "locked":
			fmt.Println("Bitwarden locked, unlocking...")
			sessionToken, err = unlockRaw()
			if err != nil {
				return "", fmt.Errorf("unlock failed: %w", err)
			}
			err = writeBWSession(sessionToken)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to cache session: %v\n", err)
			}
		case "unlocked":
			if cachedToken != "" {
				return cachedToken, nil
			}
			return "", fmt.Errorf("vault unlocked but no session token found")
		default:
			return "", fmt.Errorf("unknown bitwarden status: %w", err)
		}

		return string(sessionToken), nil
	}

	return "", nil
}
