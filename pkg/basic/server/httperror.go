package server

import (
	"fmt"
	"net/http"
)

type HTTPError struct {
	Code  int
	Inner error
}

func (e HTTPError) Error() string {
	statusText := http.StatusText(e.Code)
	if statusText != "" {
		statusText = " " + statusText
	}
	return fmt.Sprintf("HTTP %d%s: %s", e.Code, statusText, e.Inner.Error())
}
