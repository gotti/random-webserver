package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"github.com/microcosm-cc/bluemonday"
)

type OpenAIClient struct {
  client *http.Client
  apiKey string
  model  string
}

func NewOpenAIClient(apiKey, model string) *OpenAIClient {
  client := &http.Client{}
  return &OpenAIClient{client: client, apiKey: apiKey, model: model}
}

type openAIMessage struct {
  Role string `json:"role"`
  Content string `json:"content"`
}

type openAIRequest struct {
  Model     string `json:"model"`
  MaxTokens int    `json:"max_tokens"`
  Messages []openAIMessage `json:"messages"`
}


type openAIResponse struct{
  Id string `json:"id"`
  Object string `json:"object"`
  Created float64 `json:"created"`
  Model string `json:"model"`
  Choices []openAIChoices `json:"choices"`
}


type openAIChoices struct{
  Index float64 `json:"index"`
  Message openAIMessage `json:"message"`
  Finish_reason string `json:"finish_reason"`
}

func extractHTML(data string) string {
  start := strings.Index(data, "<!DOCTYPE html>")
  end := strings.Index(data, "</html>")
  if start == -1 || end == -1 {
    return ""
  }
  html := data[start:end+7]
  p := bluemonday.UGCPolicy()
  return p.Sanitize(html)
}

func (c *OpenAIClient) CreateCompletion(system, prompt string) (string, error) {
  reqBody := openAIRequest{
    Model:     c.model,
    MaxTokens: 810,
    Messages: []openAIMessage{
      {
        Role:    "system",
        Content: system,
      },
      {
        Role:    "user",
        Content: prompt,
      },
    },
  }
  reqJson, err := json.Marshal(reqBody)
  if err != nil {
    return "", fmt.Errorf("failed to marshal request body: %w", err)
  }
  fmt.Println(string(reqJson))

  req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqJson))
  if err != nil {
    return "", fmt.Errorf("failed to create request: %w", err)
  }
  req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
  req.Header.Set("Content-Type", "application/json")
  res, err := c.client.Do(req)
  if err != nil {
    return "", fmt.Errorf("failed to request: %w", err)
  }
  defer res.Body.Close()
  body, err := io.ReadAll(res.Body)
  if err != nil {
    return "", fmt.Errorf("failed to read response body: %w", err)
  }
  var resp openAIResponse
  err = json.Unmarshal(body, &resp)
  if err != nil {
    return "", fmt.Errorf("failed to unmarshal response body: %w", err)
  }
  if len(resp.Choices) == 0 {
    return "", fmt.Errorf("failed to get response: %s", string(body))
  }
  d := resp.Choices[0].Message.Content
  return extractHTML(d), nil
}

func main() {
  // get apikey from env API_KEY
  apikey := os.Getenv("API_KEY")
  client := NewOpenAIClient(apikey, "gpt-3.5-turbo-1106")
  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    if strings.Contains(r.URL.Path, "favicon.ico") {
      w.WriteHeader(http.StatusNotFound)
      return
    }
    res, err := client.CreateCompletion(`Create and design an HTML document as a response to the following HTTP request.
Within this HTML document, include at least one link to related pages using the <a> tag.
Utilize multiple <div> tags for proper layout organization.
Ensure the content appropriately addresses the user's requested information.
Use Japanese as the language of the HTML document.
The output should start with <!DOCTYPE html> and should contain only HTML code.`, r.URL.Path)
    if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
      fmt.Println("failed to create completions", err)
      return
    }
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(res))
  })
  http.ListenAndServe(":11451", nil)
}
