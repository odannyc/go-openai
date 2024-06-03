package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// GetErrHTTPStatus returns the HTTP response status code that caused the given error.
// If the error was not caused by an HTTP response, it returns 0.
func GetErrHTTPStatus(err error) int {
	apiErr := new(APIError)
	reqErr := new(RequestError)
	switch {
	case errors.As(err, &apiErr):
		return apiErr.HTTPStatusCode
	case errors.As(err, &reqErr):
		return reqErr.HTTPStatusCode
	}
	return 0
}

// IsTooManyRequests takes an error returned by a client and check whether the error indicates that the
// client got a 429 "Too Many Requests" response from the server.
func IsTooManyRequests(err error) (is429 bool, retryAfter string) {
	apiErr := new(APIError)
	reqErr := new(RequestError)
	switch {
	case errors.As(err, &apiErr) && apiErr.HTTPStatusCode == http.StatusTooManyRequests:
		return true, apiErr.HTTPRetryAfter
	case errors.As(err, &reqErr) && reqErr.HTTPStatusCode == http.StatusTooManyRequests:
		return true, reqErr.HTTPRetryAfter
	}
	return false, ""
}

// APIError provides error information returned by the OpenAI API.
// InnerError struct is only valid for Azure OpenAI Service.
type APIError struct {
	Code           any         `json:"code,omitempty"`
	Message        string      `json:"message"`
	Param          *string     `json:"param,omitempty"`
	Type           string      `json:"type"`
	HTTPStatusCode int         `json:"-"`
	HTTPRetryAfter string      `json:"-"`
	InnerError     *InnerError `json:"innererror,omitempty"`
}

// InnerError Azure Content filtering. Only valid for Azure OpenAI Service.
type InnerError struct {
	Code                 string               `json:"code,omitempty"`
	ContentFilterResults ContentFilterResults `json:"content_filter_result,omitempty"`
}

// RequestError provides informations about generic request errors.
type RequestError struct {
	HTTPStatusCode int
	HTTPRetryAfter string
	Err            error
}

type ErrorResponse struct {
	Error *APIError `json:"error,omitempty"`
}

func (e *APIError) Error() string {
	if e.HTTPStatusCode > 0 {
		return fmt.Sprintf("error, status code: %d, message: %s", e.HTTPStatusCode, e.Message)
	}

	return e.Message
}

func (e *APIError) UnmarshalJSON(data []byte) (err error) {
	var rawMap map[string]json.RawMessage
	err = json.Unmarshal(data, &rawMap)
	if err != nil {
		return
	}

	err = json.Unmarshal(rawMap["message"], &e.Message)
	if err != nil {
		// If the parameter field of a function call is invalid as a JSON schema
		// refs: https://github.com/sashabaranov/go-openai/issues/381
		var messages []string
		err = json.Unmarshal(rawMap["message"], &messages)
		if err != nil {
			return
		}
		e.Message = strings.Join(messages, ", ")
	}

	// optional fields for azure openai
	// refs: https://github.com/sashabaranov/go-openai/issues/343
	if _, ok := rawMap["type"]; ok {
		err = json.Unmarshal(rawMap["type"], &e.Type)
		if err != nil {
			return
		}
	}

	if _, ok := rawMap["innererror"]; ok {
		err = json.Unmarshal(rawMap["innererror"], &e.InnerError)
		if err != nil {
			return
		}
	}

	// optional fields
	if _, ok := rawMap["param"]; ok {
		err = json.Unmarshal(rawMap["param"], &e.Param)
		if err != nil {
			return
		}
	}

	if _, ok := rawMap["code"]; !ok {
		return nil
	}

	// if the api returned a number, we need to force an integer
	// since the json package defaults to float64
	var intCode int
	err = json.Unmarshal(rawMap["code"], &intCode)
	if err == nil {
		e.Code = intCode
		return nil
	}

	return json.Unmarshal(rawMap["code"], &e.Code)
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("error, status code: %d, message: %s", e.HTTPStatusCode, e.Err)
}

func (e *RequestError) Unwrap() error {
	return e.Err
}
