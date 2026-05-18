package api

import "net/http"

func writeInvalidRequestError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, APIError{
		Message: message,
		Type:    "invalid_request_error",
		Code:    "invalid_request_error",
	})
}

func writeMethodNotAllowedError(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, APIError{
		Message: "Method not allowed.",
		Type:    "invalid_request_error",
		Code:    "method_not_allowed",
	})
}

func writeNotFoundError(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, APIError{
		Message: "Not found.",
		Type:    "invalid_request_error",
		Code:    "not_found",
	})
}

func writeError(w http.ResponseWriter, statusCode int, apiErr APIError) {
	writeJSON(w, statusCode, APIErrorResponse{Error: apiErr})
}
