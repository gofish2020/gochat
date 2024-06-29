/**
 * Created by nash
 * Date: 2019-08-12
 * Time: 11:17
 */
package api

import (
	"context"
	"flag"
	"fmt"
	"gochat/api/router"
	"gochat/api/rpc"
	"gochat/config"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Chat struct {
}

func New() *Chat {
	return &Chat{}
}

// api server,Also, you can use gin,echo ... framework wrap
func (c *Chat) Run() {
	// 初始化 logic 的 rpc 客户端
	rpc.InitLogicRpcClient()

	// 利用gin框架，构建http服务
	r := router.Register()

	// 获取运行模式  debug/ release
	runMode := config.GetGinRunMode()
	logrus.Info("server start , now run mode is ", runMode)
	gin.SetMode(runMode) // 设置gin

	// 对应 api.toml文件
	apiConfig := config.Conf.Api
	port := apiConfig.ApiBase.ListenPort
	flag.Parse()

	// 配置 gin服务，端口为 api.toml文件中的端口
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	go func() {
		// 启动 gin 服务（等待连接）
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("start listen : %s\n", err)
		}
	}()
	// 监视信号，等待退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	logrus.Infof("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // 5s
	defer cancel()

	//  优雅关闭（最多延迟 5s，就强制退出）
	if err := srv.Shutdown(ctx); err != nil {
		logrus.Error("Server Shutdown:", err) //超过5s也会自动退出
	}
	logrus.Infof("Server exiting")
	os.Exit(0)
}
