package utils

import (
	"encoding/json"
	"fmt"
)

func PrettyPrintMap(m map[string]any) {
	prettyJSON, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(string(prettyJSON))
}
