package https_server

import (
	v1 "github.com/afiff2/go-chat-server/api"
	"github.com/gin-gonic/gin"
)

var GE *gin.Engine

func init() {
	GE = gin.Default()

	userGroup := GE.Group("/user")
	{
		userGroup.POST("/register", v1.Register)
		userGroup.POST("/login", v1.Login)
		userGroup.POST("/delete", v1.DeleteUsers)
	}

}
