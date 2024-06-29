/**
 * Created by nash
 * Date: 2019-10-06
 * Time: 23:09
 */
package router

import (
	"gochat/api/handler"
	"gochat/api/rpc"
	"gochat/proto"
	"gochat/tools"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func Register() *gin.Engine {
	r := gin.Default()
	// 中间件
	r.Use(CorsMiddleware())
	// 注册/user路由
	initUserRouter(r)
	// 注册/push路由
	initPushRouter(r)

	// 无路由，返回报错
	r.NoRoute(func(c *gin.Context) {
		tools.FailWithMsg(c, "please check request url !")
	})
	return r
}

// 注册/user路由
func initUserRouter(r *gin.Engine) {
	userGroup := r.Group("/user")
	userGroup.POST("/login", handler.Login) // 登录
	userGroup.POST("/register", handler.Register)

	// 【注意】在这里调用 Use，说明 /checkAuth /logout，才会带上这个中间件的处理逻辑
	userGroup.Use(CheckSessionId())
	{
		userGroup.POST("/checkAuth", handler.CheckAuth)
		userGroup.POST("/logout", handler.Logout)
	}

}

// 注册/push路由
func initPushRouter(r *gin.Engine) {
	pushGroup := r.Group("/push")
	pushGroup.Use(CheckSessionId())
	{
		pushGroup.POST("/push", handler.Push)
		pushGroup.POST("/pushRoom", handler.PushRoom) // 发消息到聊天室
		pushGroup.POST("/count", handler.Count)
		pushGroup.POST("/getRoomInfo", handler.GetRoomInfo)
	}

}

type FormCheckSessionId struct {
	AuthToken string `form:"authToken" json:"authToken" binding:"required"`
}

func CheckSessionId() gin.HandlerFunc {
	return func(c *gin.Context) {
		var formCheckSessionId FormCheckSessionId
		if err := c.ShouldBindBodyWith(&formCheckSessionId, binding.JSON); err != nil {
			c.Abort()
			tools.ResponseWithCode(c, tools.CodeSessionError, nil, nil)
			return
		}
		// 获取请求中的 authToken
		authToken := formCheckSessionId.AuthToken
		req := &proto.CheckAuthRequest{
			AuthToken: authToken,
		}
		// 通过rpc 验证 authToken是否有效
		code, userId, userName := rpc.RpcLogicObj.CheckAuth(req)
		if code == tools.CodeFail || userId <= 0 || userName == "" {
			c.Abort()
			tools.ResponseWithCode(c, tools.CodeSessionError, nil, nil) // 无效，直接返回
			return
		}
		c.Next()
	}
}

func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		// 设置响应头，表示服务器允许的参数 https://www.jianshu.com/p/89a377c52b48
		c.Header("Access-Control-Allow-Origin", "*")                                               // 允许其他域名访问；因为站点是8080端口，api服务是7070端口
		c.Header("Access-Control-Max-Age", "1800")                                                 // 预检结果缓存时间
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept") // 允许的请求头字段
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS, POST, PUT, DELETE")                // 允许的请求类型
		c.Set("content-type", "application/json")                                                  // application/x-www-form-urlencoded、multipart/form-data、text/plain

		if method == "OPTIONS" { // OPTIONS 预检请求
			c.JSON(http.StatusOK, nil)
		}
		c.Next()
	}
}
