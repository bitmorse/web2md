package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func baseURL() string {
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		return v
	}
	return "https://api.openai.com/v1"
}

func apiKey() string {
	return os.Getenv("OPENAI_API_KEY")
}

func model() string {
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		return v
	}
	return "gpt-4o-mini"
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func chat(prompt string) (string, error) {
	reqBody := chatRequest{
		Model: model(),
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", baseURL()+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if key := apiKey(); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(body))
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}
	return cr.Choices[0].Message.Content, nil
}

// FilterPage asks the LLM whether the page matches the description.
// Returns true if the LLM says the page matches.
func FilterPage(pageURL, title, metaDesc, description string) (bool, error) {
	prompt := fmt.Sprintf(
		`Does the following web page match this description: "%s"?

URL: %s
Title: %s
Meta description: %s

Answer with only "yes" or "no".`,
		description, pageURL, title, metaDesc,
	)

	answer, err := chat(prompt)
	if err != nil {
		return false, err
	}

	// Accept any response starting with "yes" (case-insensitive)
	lower := bytes.ToLower([]byte(answer))
	return bytes.HasPrefix(lower, []byte("yes")), nil
}

const maxHTMLLen = 100000 // ~100KB limit for LLM input

// ConvertToMarkdown asks the LLM to convert HTML to clean Markdown.
func ConvertToMarkdown(html string) (string, error) {
	if len(html) > maxHTMLLen {
		html = html[:maxHTMLLen]
	}
	prompt := fmt.Sprintf(
		`Convert the following HTML to clean, readable Markdown. Return only the Markdown content, no explanations.

HTML:
%s`, html,
	)
	return chat(prompt)
}
