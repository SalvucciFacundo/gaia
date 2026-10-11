package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"gaia/internal/core/ports"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "tts-1"
	defaultOpenAIVoice   = "alloy"
	defaultOpenAIFormat  = "mp3"
	defaultWhisperModel  = "whisper-1"
)

// Whisper is the OpenAI Whisper audio transcription adapter implementing ports.Transcriber.
type Whisper struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

var _ ports.Transcriber = (*Whisper)(nil)

// NewWhisper creates a new Whisper transcription adapter targeting baseURL.
// If baseURL is empty, it defaults to the OpenAI v1 API base endpoint.
// If model is empty, it defaults to "whisper-1".
func NewWhisper(baseURL, apiKey, model string) *Whisper {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	if model == "" {
		model = defaultWhisperModel
	}
	return &Whisper{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Transcribe sends a multipart/form-data POST request to /audio/transcriptions
// with the provided audio bytes and returns the transcribed text.
func (w *Whisper) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	if len(audio) == 0 {
		return "", errors.New("audio payload is empty")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	filename := DeduceFilename(mimeType)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("model", w.model); err != nil {
		return "", fmt.Errorf("writing model field: %w", err)
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return "", fmt.Errorf("writing audio payload: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("closing multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/audio/transcriptions", w.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", fmt.Errorf("building transcription request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if w.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+w.apiKey)
	}

	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("posting transcription request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		var errEnv struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &errEnv) == nil && errEnv.Error.Message != "" {
			return "", fmt.Errorf("audio: transcription failed (status %d): %s", resp.StatusCode, errEnv.Error.Message)
		}
		return "", fmt.Errorf("audio: transcription failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text  string `json:"text"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding transcription response: %w", err)
	}
	if result.Error != nil && result.Error.Message != "" {
		return "", fmt.Errorf("audio: transcription error: %s", result.Error.Message)
	}

	return result.Text, nil
}

// DeduceFilename infers an appropriate audio filename with extension from a MIME type.
func DeduceFilename(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch mimeType {
	case "audio/ogg", "audio/opus", "application/ogg":
		return "audio.ogg"
	case "audio/wav", "audio/x-wav":
		return "audio.wav"
	case "audio/mpeg", "audio/mp3":
		return "audio.mp3"
	case "audio/mp4":
		return "audio.mp4"
	case "audio/x-m4a", "audio/m4a":
		return "audio.m4a"
	case "audio/webm":
		return "audio.webm"
	case "audio/flac", "audio/x-flac":
		return "audio.flac"
	default:
		return "audio.bin"
	}
}

// OpenAITTSOptions provides configuration for the OpenAI TTS adapter.
type OpenAITTSOptions struct {
	BaseURL    string
	APIKey     string
	Model      string
	Voice      string
	Format     string
	Speed      float64
	HTTPClient *http.Client
}

// OpenAITTS implements ports.Synthesizer using the OpenAI /audio/speech API.
type OpenAITTS struct {
	baseURL    string
	apiKey     string
	model      string
	voice      string
	format     string
	speed      float64
	httpClient *http.Client
}

var _ ports.Synthesizer = (*OpenAITTS)(nil)

// NewOpenAITTS constructs a new OpenAITTS synthesizer.
func NewOpenAITTS(opts OpenAITTSOptions) *OpenAITTS {
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	model := opts.Model
	if model == "" {
		model = defaultOpenAIModel
	}
	voice := opts.Voice
	if voice == "" {
		voice = defaultOpenAIVoice
	}
	format := opts.Format
	if format == "" {
		format = defaultOpenAIFormat
	}
	speed := opts.Speed
	if speed <= 0 {
		speed = 1.0
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &OpenAITTS{
		baseURL:    baseURL,
		apiKey:     opts.APIKey,
		model:      model,
		voice:      voice,
		format:     format,
		speed:      speed,
		httpClient: client,
	}
}

// Synthesize converts text into speech audio bytes.
func (o *OpenAITTS) Synthesize(ctx context.Context, text string) ([]byte, string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, "", errors.New("audio: input text is empty")
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("%s/audio/speech", o.baseURL)

	reqPayload := map[string]any{
		"model":           o.model,
		"input":           trimmed,
		"voice":           o.voice,
		"response_format": o.format,
		"speed":           o.speed,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, "", fmt.Errorf("audio: marshaling openai tts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("audio: creating openai tts request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("audio: executing openai tts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("audio: openai tts failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("audio: reading openai tts response body: %w", err)
	}

	mimeType := formatToMime(o.format)
	return audioData, mimeType, nil
}

func formatToMime(format string) string {
	switch strings.ToLower(format) {
	case "opus":
		return ports.MimeTypeOpus
	case "aac":
		return ports.MimeTypeAAC
	case "flac":
		return "audio/flac"
	case "wav":
		return ports.MimeTypeWAV
	case "mp3":
		return ports.MimeTypeMP3
	default:
		return ports.MimeTypeMP3
	}
}
