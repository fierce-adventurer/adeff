package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type AIAnalysis struct {
	Summary  string `json:"summary"`
	Language string `json:"language"`
}

// Internal struct to parse Groq's HTTP response
type groqHTTPResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func AnalyzeBookText(text string) (*AIAnalysis, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY is not set")
	}

	url := "https://api.groq.com/openai/v1/chat/completions"

	payload := map[string]interface{}{
		"model": "llama-3.1-8b-instant",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a literary analyzer. Read the provided text. You must respond in pure JSON format containing exactly two keys: 'summary' (a 2-sentence summary of the text) and 'language' (detect the language of the text and return strictly 'ENGLISH' or 'HINDI').",
			},
			{
				"role":    "user",
				"content": text,
			},
		},
		"temperature": 0.1,
		"response_format": map[string]string{
			"type": "json_object", // Forces Groq to output valid JSON
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result groqHTTPResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned from Groq")
	}

	// Now we parse Groq's text output into our struct
	var analysis AIAnalysis
	rawContent := result.Choices[0].Message.Content

	if err := json.Unmarshal([]byte(rawContent), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse Groq JSON output: %w", err)
	}

	// Normalize just in case Groq adds weird casing
	analysis.Language = strings.ToUpper(strings.TrimSpace(analysis.Language))

	return &analysis, nil
}
