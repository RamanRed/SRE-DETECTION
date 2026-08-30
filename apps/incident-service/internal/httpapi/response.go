package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
)

const maxRequestBody = 2 << 20

type errorResponse struct {
	Status      int               `json:"status"`
	Message     string            `json:"message"`
	Timestamp   string            `json:"timestamp"`
	FieldErrors map[string]string `json:"fieldErrors,omitempty"`
}

func (h *Handler) decodeJSON(response http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBody)
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(destination); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return fmt.Errorf("invalid JSON request body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func (h *Handler) writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(value); err != nil {
		h.logger.Error("encode HTTP response", "error", err)
	}
}

func (h *Handler) writeError(response http.ResponseWriter, status int, message string, fields map[string]string) {
	h.writeJSON(response, status, errorResponse{
		Status: status, Message: message, Timestamp: localTime(h.clock()), FieldErrors: fields,
	})
}

func (h *Handler) writeServiceError(response http.ResponseWriter, err error) {
	var serviceError *service.Error
	if errors.As(err, &serviceError) {
		switch serviceError.Code {
		case service.CodeInvalid:
			h.writeError(response, http.StatusBadRequest, serviceError.Message, nil)
		case service.CodeNotFound:
			h.writeError(response, http.StatusNotFound, serviceError.Message, nil)
		case service.CodeConflict:
			h.writeError(response, http.StatusConflict, serviceError.Message, nil)
		case service.CodeUnavailable:
			h.writeError(response, http.StatusServiceUnavailable, serviceError.Message, nil)
		default:
			h.writeError(response, http.StatusInternalServerError, "An unexpected error occurred", nil)
		}
		return
	}
	h.logger.Error("HTTP request failed", "error", err)
	h.writeError(response, http.StatusInternalServerError, "An unexpected error occurred", nil)
}

func parsePagination(request *http.Request) (int, int, error) {
	page, err := queryInteger(request, "page", 0)
	if err != nil {
		return 0, 0, err
	}
	size, err := queryInteger(request, "size", 20)
	if err != nil {
		return 0, 0, err
	}
	return page, size, nil
}

func queryInteger(request *http.Request, name string, fallback int) (int, error) {
	value := request.URL.Query().Get(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func maxCharacters(value string) int {
	return utf8.RuneCountInString(value)
}
