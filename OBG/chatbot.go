package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Define the API URL
	apiURL := "http://localhost:11434/api/generate"

	// Create the payload
	prompt := "Hello, how are you?"
	model := "llama3" // Specify the model name
	payload := map[string]string{
		"model":  model,
		"prompt": prompt,
	}

	// Serialize payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	// Make the POST request
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Fatalf("Failed to make POST request: %v", err)
	}
	defer resp.Body.Close()

	// Handle streaming or non-standard JSON response
	scanner := bufio.NewScanner(resp.Body)
	fmt.Println("Response from Ollama:")
	var response string
	for scanner.Scan() {
		line := scanner.Text()
		var parsedLine map[string]interface{}
		err := json.Unmarshal([]byte(line), &parsedLine)
		if err != nil {
			// If parsing fails, assume it's plain text
			fmt.Println(line)
		} else if resp, ok := parsedLine["response"]; ok {
			// Print human-readable response if "response" key exists
			response=response+fmt.Sprintf("%v", resp) //resp is an interface{} type, so I must convert to string
		} else {
			// Print entire JSON object as fallback
			fmt.Print(parsedLine)
		}
	}
	fmt.Println(response)//send back full response
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading response: %v", err)
	}
}

