package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type apiErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func readAPIError(resp *http.Response, fallback string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s", fallback)
	}
	if len(body) == 0 {
		return fmt.Errorf("%s", fallback)
	}
	var payload apiErrorResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = strings.TrimSpace(payload.Error)
		}
		if message != "" {
			return fmt.Errorf("%s", message)
		}
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s", fallback)
	}
	return fmt.Errorf("%s", message)
}
