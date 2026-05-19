package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func AnalyzeBookText(text string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY is not set in environment variables")
	}

	url := "https://api.groq.com/openai/v1/chat/completions"

	//defining request payload
	payload := map[string]interface{}{
		"model": "llama-3.1-8b-instant",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a literary analyzer for an audiobook platform. Read the provided text extracted from the beginning of a book. Reply ONLY with a short, 2-sentence summary of what the book appears to be about based on this text.",
			},
			{

				"role":    "user",
				"content": text,
			},
		},
		"temperature": 0.3,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %v", err)
	}

	//create http request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	//execute request

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-200 response from Groq API: %d - %s", resp.StatusCode, string(body))
	}

	var result GroqResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal Groq response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from Groq API")
	}

	return result.Choices[0].Message.Content, nil

}
