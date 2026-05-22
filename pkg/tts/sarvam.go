package tts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type SarvamRequest struct {
	Inputs              []string `json:"inputs"`
	TargetLanguageCode  string   `json:"target_language_code"`
	Speaker             string   `json:"speaker"`
	Pace                float64  `json:"pace"`
	SpeechSampleRate    int      `json:"speech_sample_rate"`
	EnablePreprocessing bool     `json:"enable_preprocessing"`
	Model               string   `json:"model"`
}

type SarvamResponse struct {
	Audios []string `json:"audios"`
}

func GenerateAudio(text string, language string) ([]byte, error) {
	apiKey := os.Getenv("SARVAM_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("SARVAM_API_KEY is not set")
	}

	// Map to Sarvam's language codes
	targetCode := "en-IN" // Default to Indian English
	if language == "HINDI" {
		targetCode = "hi-IN"
	}

	payload := SarvamRequest{
		Inputs:              []string{text},
		TargetLanguageCode:  targetCode,
		Speaker:             "ritu", // Sarvam's standard female voice
		Pace:                1.05,   // Slightly faster for audiobook cadence
		SpeechSampleRate:    22050,  // High quality audio
		EnablePreprocessing: true,
		Model:               "bulbul:v3",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.sarvam.ai/text-to-speech", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("api-subscription-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sarvam API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result SarvamResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Audios) == 0 {
		return nil, fmt.Errorf("no audio returned from Sarvam")
	}

	// Sarvam returns a base64 encoded string. We decode it to raw audio bytes.
	audioBytes, err := base64.StdEncoding.DecodeString(result.Audios[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 audio: %w", err)
	}

	return audioBytes, nil
}
