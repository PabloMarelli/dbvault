package vault

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type VaultConfig struct {
	VaultURL    string `json:"vault_url"`
	DatabaseURL string `json:"database_url"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

type VaultRequest struct {
	VaultAddress string `json:"vault_address"`
	VaultUser    string `json:"vault_user"`
	Password     string `json:"password"`
}

type VaultResponse struct {
	RequestID string `json:"request_id"`
	Auth      Auth   `json:"auth"`
	Data      Data   `json:"data"`
}

type VaultResponseError struct {
	Errors []string `json:"errors,omitempty"`
	Error  string   `json:"error,omitempty"`
}

func (e *VaultResponseError) HasErrors() bool {
	return len(e.Errors) > 0 || e.Error != ""
}

func (e *VaultResponseError) FormatError() string {
	if e.Error != "" {
		return e.Error
	}
	if len(e.Errors) > 0 {
		return fmt.Sprintf("%v", e.Errors)
	}
	return "unknown error"
}

type VaultDatabaseResponse struct {
	Data struct {
		Keys []string `json:"keys"`
	} `json:"data"`
}

type Auth struct {
	ClientToken   string `json:"client_token"`
	LeaseDuration int    `json:"lease_duration"`
}

type Data struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type DBConnections struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type VaultConnectionConfigResponse struct {
	Data struct {
		AllowedRoles      []string `json:"allowed_roles"`
		ConnectionDetails struct {
			ConnectionURL string `json:"connection_url"`
			Username      string `json:"username"`
		} `json:"connection_details"`
		PluginName    string `json:"plugin_name"`
		PluginVersion string `json:"plugin_version"`
	} `json:"data"`
}

type Database struct {
	Name        string
	Environment string
}

func HTTPRequest(method, url string, body, headers map[string]any) ([]byte, error) {
	client := &http.Client{}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("error while marshaling the json: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error while assembling the new vault request: %w", err)
	}

	for key, value := range headers {
		if str, ok := value.(string); ok {
			req.Header.Add(key, str)
		}
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error while running the new vault request: %w", err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading the response body: %w", err)
	}

	var vaultErr VaultResponseError
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if unmarshalErr := json.Unmarshal(resBody, &vaultErr); unmarshalErr == nil && vaultErr.HasErrors() {
			return nil, fmt.Errorf("HTTP %d: Vault error: %s", res.StatusCode, vaultErr.FormatError())
		}
		return nil, fmt.Errorf("HTTP %d: request failed", res.StatusCode)
	}
	if unmarshalErr := json.Unmarshal(resBody, &vaultErr); unmarshalErr == nil && vaultErr.HasErrors() {
		return nil, fmt.Errorf("HTTP %d: Vault returned errors: %s", res.StatusCode, vaultErr.FormatError())
	}

	return resBody, nil
}

func GetVaultToken(vaultConfig VaultConfig) (string, error) {
	loginURL := fmt.Sprintf("%s/v1/auth/userpass/login/%s", vaultConfig.VaultURL, vaultConfig.Username)

	vaultReqBody := map[string]any{
		"password": vaultConfig.Password,
	}
	headers := map[string]any{
		"Content-Type": "application/json",
	}

	res, err := HTTPRequest("POST", loginURL, vaultReqBody, headers)
	if err != nil {
		return "", fmt.Errorf("error while retrieving the vault token: %w", err)
	}

	var response VaultResponse
	err = json.Unmarshal(res, &response)
	if err != nil {
		return "", fmt.Errorf("error while unmarshalling vault response: %w", err)
	}
	return string(response.Auth.ClientToken), nil
}

func GetDatabaseConnectionURL(vaultConfig VaultConfig, token, dbName string) (string, error) {
	configURL := fmt.Sprintf("%s/v1/database/config/%s", vaultConfig.VaultURL, dbName)

	headers := map[string]any{
		"Content-Type":  "application/json",
		"X-Vault-Token": token,
	}

	res, err := HTTPRequest("GET", configURL, map[string]any{}, headers)
	if err != nil {
		return "", fmt.Errorf("error retrieving database config: %w", err)
	}

	var response VaultConnectionConfigResponse
	err = json.Unmarshal(res, &response)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling config response: %w", err)
	}

	return response.Data.ConnectionDetails.ConnectionURL, nil
}

func GetDatabaseCredentials(vaultConfig VaultConfig, token, selected string) (VaultResponse, error) {
	cleanDBName := strings.Replace(selected, "jangl-", "", 1)
	loginURL := fmt.Sprintf("%s/v1/database/static-creds/ops-%s", vaultConfig.VaultURL, cleanDBName)

	vaultReqBody := map[string]any{}
	headers := map[string]any{
		"Content-Type":  "application/json",
		"X-Vault-Token": token,
	}

	res, err := HTTPRequest("GET", loginURL, vaultReqBody, headers)
	if err != nil {
		return VaultResponse{}, fmt.Errorf("error in the http request while retrieving the credentials: %w", err)
	}

	var credentials VaultResponse
	err = json.Unmarshal(res, &credentials)
	if err != nil {
		return VaultResponse{}, fmt.Errorf("error in unmarshalling the credentials: %w", err)
	}

	return credentials, nil
}

func GetVaultDatabaseList(vaultConfig VaultConfig, env string) ([]Database, error) {
	configURL := fmt.Sprintf("%s/v1/database/config?list=true", vaultConfig.VaultURL)

	token, err := GetVaultToken(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("error while getting the vault token: %w", err)
	}

	vaultReqBody := map[string]any{}
	headers := map[string]any{
		"Content-Type":  "application/json",
		"X-Vault-Token": token,
	}
	res, err := HTTPRequest("LIST", configURL, vaultReqBody, headers)
	if err != nil {
		return nil, fmt.Errorf("error while doing vault database list request: %w", err)
	}

	var vaultDatabaseList VaultDatabaseResponse
	err = json.Unmarshal(res, &vaultDatabaseList)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling vault database list: %w", err)
	}
	var databaseList []Database
	for _, v := range vaultDatabaseList.Data.Keys {
		databaseList = append(databaseList, Database{Name: v, Environment: env})
	}

	return databaseList, nil
}

func GetDatabaseList(env, bwToken string, vaultConfig VaultConfig) ([]Database, error) {
	var databases []Database
	var err error
	switch env {
	case "prod":
		databases, err = GetVaultDatabaseList(vaultConfig, env)
		if err != nil {
			return nil, fmt.Errorf("error while retrieving the database list from vault: %w", err)
		}
	case "sqa":
		// AWS
		fmt.Println("AWS credentials retrieving feature is in development")
	case "dev":
		// AWS
		fmt.Println("AWS credentials retrieving feature is in development")
	}
	return databases, nil
}

func UpdateNvimDB(nvimDBPath string, dbCredentials VaultResponse, environment, selected, fullURL string) error {
	dbFile, err := os.ReadFile(nvimDBPath)
	if err != nil {
		return fmt.Errorf("error opening db connections file: %w", err)
	}

	var nvimDB []DBConnections
	err = json.Unmarshal(dbFile, &nvimDB)
	if err != nil {
		return fmt.Errorf("error parsing the db connections file: %w", err)
	}

	searchName := fmt.Sprintf("%s-%s", environment, selected)
	found := false

	for i, conn := range nvimDB {
		if conn.Name == searchName {
			nvimDB[i].URL = fullURL
			found = true
			fmt.Printf("✓ Updated connection: %s\n", searchName)
			break
		}
	}
	if !found {
		newConn := DBConnections{
			Name: searchName,
			URL:  fullURL,
		}
		nvimDB = append(nvimDB, newConn)
		fmt.Printf("✓ Added new connection: %s\n", searchName)

	}

	updatedData, err := json.MarshalIndent(nvimDB, "", "  ")
	if err != nil {
		return fmt.Errorf("error while marshal indenting the connections: %w", err)
	}

	return os.WriteFile(nvimDBPath, updatedData, 0644)
}
