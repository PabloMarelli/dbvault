package bitwarden

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/term"
)

type BitwardenItem struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Type   int     `json:"type"`
	Login  Login   `json:"login"`
	Fields []Field `json:"fields"`
}

type Login struct {
	Username             string  `json:"username"`
	Password             string  `json:"password"`
	PasswordRevisionDate *string `json:"passwordRevisionDate"`
	TOTP                 *string `json:"totp"`
	URIs                 []URI   `json:"uris"`
}

type URI struct {
	Match *string `json:"match"`
	URI   string  `json:"uri"`
}

type Field struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Type     int     `json:"type"`
	LinkedID *string `json:"linkedId"`
}

func GetOrCreateBWItem(bwItem, bwSession string) (*BitwardenItem, error) {
	fmt.Println("Retrieving BW item")

	getBWItemCmd := exec.Command("bw", "get", "item", bwItem, "--session", bwSession)
	out, _ := getBWItemCmd.CombinedOutput()

	if string(out) == "Not found." {
		newBWItem, err := CreateBWItem(bwItem, bwSession)
		if err != nil {
			return nil, fmt.Errorf("error while creating the new bw item: %w", err)
		}
		return newBWItem, nil
	}

	var item BitwardenItem
	err := json.Unmarshal(out, &item)
	if err != nil {
		return nil, fmt.Errorf("error Unmarshalling JSON: %w", err)
	}
	return &item, nil
}

func GetBWTemplateItem(bwSession string) (*BitwardenItem, error) {
	getBWItemCmd := exec.Command("bw", "get", "template", "item", "--session", bwSession)
	out, err := getBWItemCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error while running the get bw item command: %w", err)
	}

	var item BitwardenItem
	err = json.Unmarshal(out, &item)
	if err != nil {
		return nil, fmt.Errorf("error Unmarshalling JSON: %w", err)
	}
	return &item, nil
}

func CreateBWItem(bwItem, bwSession string) (*BitwardenItem, error) {
	fmt.Println("Creating BW item.")
	fmt.Println("Enter Vault username: ")
	var username string
	fmt.Scan(&username)

	fmt.Println("Enter Vault password: ")
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}

	templateItem, err := GetBWTemplateItem(bwSession)
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}
	templateItem.Name = bwItem
	templateItem.Login.Username = username
	templateItem.Login.Password = string(bytePassword)
	fmt.Println(templateItem)

	encodeCmd := exec.Command("bw", "encode", "--session", bwSession)
	modifiedJSON, err := json.Marshal(templateItem)
	if err != nil {
		return nil, fmt.Errorf("error marshalling template item: %w", err)
	}

	encodeCmd.Stdin = bytes.NewReader(modifiedJSON)
	encodedData, err := encodeCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Encode error: %v\nOutput: %s\n", err, encodedData)
		return nil, fmt.Errorf("failed to encode: %w", err)
	}

	createCmd := exec.Command("bw", "create", "item", "--session", bwSession)
	createCmd.Stdin = bytes.NewReader(encodedData)
	result, err := createCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Create error: %v\nOutput: %s\n", err, result)
		return nil, fmt.Errorf("failed to create: %w", err)
	}

	fmt.Println(string(result))

	return templateItem, nil
}
