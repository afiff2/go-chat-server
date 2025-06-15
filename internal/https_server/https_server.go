package https_server

import (
	v1 "github.com/afiff2/go-chat-server/api"
	"github.com/gin-gonic/gin"
)

var GinEngine *gin.Engine

func init() {
	GinEngine = gin.Default()

	userGroup := GinEngine.Group("/user")
	{
		userGroup.POST("/register", v1.Register)
		userGroup.POST("/login", v1.Login)
		userGroup.POST("/delete", v1.DeleteUsers)
		userGroup.POST("/get", v1.GetUserInfo)
		userGroup.POST("/update", v1.UpdateUserInfo)
	}

}
