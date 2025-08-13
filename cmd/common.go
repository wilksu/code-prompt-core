package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func printJSON(data interface{}) {
	resp := Response{Status: "success", Data: data}
	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		printError(fmt.Errorf("failed to marshal JSON response: %w", err))
		return
	}
	fmt.Println(string(bytes))
}

func printError(err error) {
	resp := ErrorResponse{Status: "error", Message: err.Error()}
	bytes, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintln(os.Stderr, string(bytes))
	os.Exit(1)
}
