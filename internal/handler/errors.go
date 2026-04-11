package handler

import "github.com/gin-gonic/gin"

// ErrorBody is the standard JSON error envelope for every endpoint.
// swagger:model ErrorResponse
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a machine-readable code and human-readable message.
type ErrorDetail struct {
	Code    string `json:"code" example:"INVALID_CREDENTIALS"`
	Message string `json:"message" example:"The provided credentials are invalid."`
}

// writeError writes a JSON error response with the given status code.
// Internal details must never be leaked — callers pass only safe messages.
func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{
		Error: ErrorDetail{Code: code, Message: message},
	})
}
