package main

import (
	"fmt"
	"os"
	"strings"

	bitwarden "vault/cmd/bitwarden"
	vault "vault/cmd/vault"

	"github.com/atotto/clipboard"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"
)

var (
	envFlag       string
	urlFlag       bool
	setNvimDBFlag bool
	BwItem        string
)

func selectEnvironment(environments []string) (*string, error) {
	index, err := fuzzyfinder.Find(environments, func(i int) string {
		return fmt.Sprintf(environments[i])
	})
	if err != nil {
		return nil, err
	}
	return &environments[index], nil
}

func selectDatabase(databases []vault.Database) (*vault.Database, error) {
	index, err := fuzzyfinder.Find(
		databases,
		func(i int) string {
			return fmt.Sprintf("%s (%s)", databases[i].Name, databases[i].Environment)
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return fmt.Sprintf("Database: %s\nEnvironment: %s\n", databases[i].Name, databases[i].Environment)
		}),
	)
	if err != nil {
		return nil, err
	}
	return &databases[index], nil
}

func getNvimDBPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error while getting homeDir: %s", err)
		return "", err
	}
	nvimDBPath := homeDir + "/.config/dbqueries/connections.json"
	return nvimDBPath, nil
}

var credentialsCmd = &cobra.Command{
	Use:   "dbvault [url] [setNvimDB]",
	Short: "Retrieve credentials from Hashicorp Vault",
	Run:   root,
}

func init() {
	credentialsCmd.Flags().StringVar(&envFlag, "env", "", "Environment")
	credentialsCmd.Flags().BoolVarP(&urlFlag, "url", "u", false, "Copy full URL to the clipboard")
	credentialsCmd.Flags().BoolVarP(&setNvimDBFlag, "setNvimDB", "n", false, "Set the NVIM DB with new password")
}

func root(cmd *cobra.Command, args []string) {
	fmt.Println("Credentials Retriever")

	environments := []string{
		"prod",
		"sqa",
		"dev",
	}

	selectedEnv, err := selectEnvironment(environments)
	if err != nil {
		fmt.Printf("Selection cancelled: %v\n", err)
		return
	}
	fmt.Printf("Selected env: %v\n", *selectedEnv)
	bwItem := fmt.Sprintf("vault-%s", *selectedEnv)

	sessionToken, err := bitwarden.GetSessionOrLogin()
	if err != nil {
		fmt.Println(err)
		return
	}

	item, err := bitwarden.GetOrCreateBWItem(bwItem, sessionToken)
	if err != nil {
		fmt.Printf("error getting the bw item: %v\n", err)
		return
	}

	config := vault.VaultConfig{}

	for _, field := range item.Fields {
		switch field.Name {
		case "URL":
			config.VaultURL = field.Value
		case "DB-URL":
			config.DatabaseURL = field.Value
		}
	}
	config.Username = item.Login.Username
	config.Password = item.Login.Password

	token, err := vault.GetVaultToken(config)
	if err != nil {
		fmt.Printf("error retrieving the token from vault: %v\n", err)
		return
	}

	databases, err := vault.GetDatabaseList(*selectedEnv, sessionToken, config)
	if err != nil {
		fmt.Printf("error while getting the database list: %v\n", err)
		return
	}

	selected, err := selectDatabase(databases)
	if err != nil {
		fmt.Printf("Selection cancelled: %v\n", err)
		return
	}
	fmt.Printf("Selected: %s\n", selected.Name)

	connectionURL, err := vault.GetDatabaseConnectionURL(config, token, selected.Name)
	if err != nil {
		fmt.Printf("error retrieving connection URL: %v\n", err)
		return
	}

	dbCredentials, err := vault.GetDatabaseCredentials(config, token, selected.Name)
	if err != nil {
		fmt.Printf("error retrieving the db credentials from vault: %v\n", err)
		return
	}

	replacer := strings.NewReplacer(
		"{{username}}", dbCredentials.Data.Username,
		"{{password}}", dbCredentials.Data.Password,
		"{{database}}", selected.Name,
	)
	fullURL := replacer.Replace(connectionURL)

	if urlFlag {
		err = clipboard.WriteAll(fullURL)
		if err != nil {
			fmt.Printf("Error copying URL to clipboard: %v\n", err)
			return
		} else {
			fmt.Println("✓ Full database URL copied to clipboard")
		}
	} else {
		err = clipboard.WriteAll(dbCredentials.Data.Password)
		if err != nil {
			fmt.Printf("Error copying password to clipboard: %v\n", err)
			return
		} else {
			fmt.Println("✓ Password copied to clipboard")
		}
	}

	if setNvimDBFlag {
		nvimDbPath, _ := getNvimDBPath()
		err = vault.UpdateNvimDB(nvimDbPath, dbCredentials, selected.Environment, selected.Name, fullURL)
		if err != nil {
			fmt.Printf("error updating the nvim db: %s", err)
			return
		}
		fmt.Println("✓ Updated nvim DB successfully")
	}
}

func main() {
	if err := credentialsCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
