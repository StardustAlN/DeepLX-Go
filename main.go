package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	DeeplApiEndpoint = "https://ideepl.vercel.app/jsonrpc"
	MaxAlternatives  = 3
)

type RequestConfig struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	ID      int64  `json:"id"`
	Params  struct {
		Texts []struct {
			Text                string `json:"text"`
			RequestAlternatives int    `json:"requestAlternatives"`
		} `json:"texts"`
		Timestamp int64  `json:"timestamp"`
		Splitting string `json:"splitting"`
		Lang      struct {
			SourceLangUserSelected string `json:"source_lang_user_selected"`
			TargetLang             string `json:"target_lang"`
		} `json:"lang"`
	} `json:"params"`
}

type TranslateParams struct {
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type TranslateResponse struct {
	Code         int      `json:"code"`
	Message      string   `json:"message"`
	Data         string   `json:"data,omitempty"`
	SourceLang   string   `json:"source_lang,omitempty"`
	TargetLang   string   `json:"target_lang,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
}

func createRequestConfig(sourceLang, targetLang string) RequestConfig {
	if sourceLang == "" {
		sourceLang = "auto"
	}
	if targetLang == "" {
		targetLang = "en"
	}

	config := RequestConfig{
		Jsonrpc: "2.0",
		Method:  "LMT_handle_texts",
		ID:      rand.Int63n(100000) + 100000*1000,
	}

	config.Params.Texts = []struct {
		Text                string `json:"text"`
		RequestAlternatives int    `json:"requestAlternatives"`
	}{{
		Text:                "",
		RequestAlternatives: MaxAlternatives,
	}}
	config.Params.Splitting = "newlines"
	config.Params.Lang.SourceLangUserSelected = strings.ToUpper(sourceLang)
	config.Params.Lang.TargetLang = strings.ToUpper(targetLang)

	return config
}

func calculateTimestamp(text string) int64 {
	timestamp := time.Now().UnixMilli()
	count := int64(strings.Count(text, "i"))

	if count != 0 {
		return timestamp - (timestamp % (count + 1)) + (count + 1)
	}
	return timestamp
}

func buildRequestBody(params TranslateParams) (string, error) {
	config := createRequestConfig(params.SourceLang, params.TargetLang)
	config.Params.Texts[0].Text = params.Text
	config.Params.Timestamp = calculateTimestamp(params.Text)

	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request config: %w", err)
	}
	body := string(jsonBytes)

	if (config.ID+5)%29 == 0 || (config.ID+5)%29 == 3 || (config.ID+3)%13 == 0 {
		body = strings.Replace(body, `"method":"`, `"method" : "`, 1)
	} else {
		body = strings.Replace(body, `"method":"`, `"method": "`, 1)
	}

	return body, nil
}

func translate(params TranslateParams) TranslateResponse {
	if params.Text == "" {
		return TranslateResponse{
			Code:    404,
			Message: "No Translate Text Found",
		}
	}

	body, err := buildRequestBody(params)
	if err != nil {
		log.Printf("Error building request body: %v", err)
		return TranslateResponse{
			Code:    500,
			Message: "Failed to build request body",
		}
	}

	resp, err := http.Post(
		DeeplApiEndpoint,
		"application/json; charset=utf-8",
		strings.NewReader(body),
	)
	if err != nil {
		log.Printf("Error making HTTP request: %v", err)
		return TranslateResponse{
			Code:    500,
			Message: "Request failed",
		}
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var result struct {
			Result struct {
				Texts []struct {
					Text         string `json:"text"`
					Alternatives []struct {
						Text string `json:"text"`
					} `json:"alternatives"`
				} `json:"texts"`
			} `json:"result"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Printf("Error decoding response: %v", err)
			return TranslateResponse{
				Code:    500,
				Message: "Failed to decode response",
			}
		}

		alternatives := make([]string, 0)
		if len(result.Result.Texts) > 0 && len(result.Result.Texts[0].Alternatives) > 0 {
			for _, alt := range result.Result.Texts[0].Alternatives {
				alternatives = append(alternatives, alt.Text)
			}
		}

		return TranslateResponse{
			Code:         200,
			Message:      "success",
			Data:         result.Result.Texts[0].Text,
			SourceLang:   params.SourceLang,
			TargetLang:   params.TargetLang,
			Alternatives: alternatives,
		}
	}

	message := "Unknown error."
	if resp.StatusCode == 429 {
		message = "Too many requests, please try again later."
	}

	return TranslateResponse{
		Code:    resp.StatusCode,
		Message: message,
	}
}

func main() {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Developed by StardustAlN. More info: https://github.com/StardustAlN/DeepLX-Go")
	})

	app.Get("/translate", func(c *fiber.Ctx) error {
		return c.SendString("Please use POST method :)")
	})

	app.Post("/translate", func(c *fiber.Ctx) error {
		var params TranslateParams
		if err := c.BodyParser(&params); err != nil {
			log.Printf("Error parsing request body: %v", err)
			return c.Status(400).JSON(TranslateResponse{
				Code:    400,
				Message: "Invalid request body",
			})
		}

		result := translate(params)
		return c.Status(result.Code).JSON(result)
	})

	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
