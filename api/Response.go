package v1

import (
	"net/http"

	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func SendResponse(c *gin.Context, message string, ret int, data interface{}) {
	var statusCode int
	var httpCode int

	switch ret {
	case constants.BizCodeSuccess:
		statusCode = http.StatusOK
		httpCode = 200
	case constants.BizCodeInvalid:
		statusCode = http.StatusBadRequest
		data = nil
		httpCode = 400
	case constants.BizCodeError:
		statusCode = http.StatusInternalServerError
		data = nil
		httpCode = 500
	default:
		statusCode = http.StatusInternalServerError
		data = nil
		httpCode = 500
		message = "未知错误"
	}

	c.JSON(statusCode, Response{
		Code:    httpCode,
		Message: message,
		Data:    data,
	})
}
