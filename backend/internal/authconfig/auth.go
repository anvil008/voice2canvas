// Package authconfig translates the supported environment variables into a
// Gen AI client configuration without making authentication a server-startup
// requirement.
package authconfig

import (
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

type Config struct {
	APIKey   string
	Project  string
	Location string
}

func FromEnv() Config {
	location := strings.TrimSpace(os.Getenv("GOOGLE_CLOUD_LOCATION"))
	if location == "" {
		location = strings.TrimSpace(os.Getenv("GOOGLE_CLOUD_REGION"))
	}
	return Config{
		APIKey:   strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		Project:  strings.TrimSpace(os.Getenv("GOOGLE_CLOUD_PROJECT")),
		Location: location,
	}
}

func (c Config) AuthConfigured() bool {
	return c.APIKey != "" || (c.Project != "" && c.Location != "")
}

// ClientConfig returns Gemini API configuration when GEMINI_API_KEY is set,
// otherwise Vertex AI configuration that lets the Gen AI SDK discover ADC.
func (c Config) ClientConfig() (*genai.ClientConfig, error) {
	if c.APIKey != "" {
		return &genai.ClientConfig{
			APIKey:  c.APIKey,
			Backend: genai.BackendGeminiAPI,
		}, nil
	}
	if c.Project == "" || c.Location == "" {
		return nil, fmt.Errorf("missing GEMINI_API_KEY or Google ADC configuration (set GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION/GOOGLE_CLOUD_REGION)")
	}
	return &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  c.Project,
		Location: c.Location,
	}, nil
}

func MissingMessage() string {
	return "missing GEMINI_API_KEY or Google ADC configuration (set GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION/GOOGLE_CLOUD_REGION)"
}
