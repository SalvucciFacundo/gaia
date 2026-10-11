package audio

import (
	"strings"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// NewSynthesizer constructs the configured ports.Synthesizer based on domain.TTSConfig.
// If TTS is disabled, it returns nil.
func NewSynthesizer(cfg domain.TTSConfig, fallbackAPIKey string) ports.Synthesizer {
	if !cfg.Enabled {
		return nil
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = fallbackAPIKey
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "elevenlabs":
		return NewElevenLabsTTS(ElevenLabsOptions{
			BaseURL: cfg.BaseURL,
			APIKey:  apiKey,
			ModelID: cfg.Model,
			VoiceID: cfg.Voice,
		})
	case "kokoro":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:8880/v1"
		}
		return NewOpenAITTS(OpenAITTSOptions{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   cfg.Model,
			Voice:   cfg.Voice,
			Format:  cfg.Format,
			Speed:   cfg.Speed,
		})
	default: // "openai" or empty
		return NewOpenAITTS(OpenAITTSOptions{
			BaseURL: cfg.BaseURL,
			APIKey:  apiKey,
			Model:   cfg.Model,
			Voice:   cfg.Voice,
			Format:  cfg.Format,
			Speed:   cfg.Speed,
		})
	}
}

// NewTranscriber constructs the configured ports.Transcriber based on domain.AudioConfig.
// If Audio is disabled, it returns nil.
func NewTranscriber(cfg domain.AudioConfig, fallbackAPIKey string) ports.Transcriber {
	if !cfg.Enabled {
		return nil
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = fallbackAPIKey
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	default: // "openai" or default
		model := cfg.Model
		if model == "" {
			model = "whisper-1"
		}
		return NewWhisper(cfg.BaseURL, apiKey, model)
	}
}
