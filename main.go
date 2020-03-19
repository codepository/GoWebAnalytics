package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/codepository/GoWebAnalytics/router"

	"github.com/codepository/GoWebAnalytics/connmgr"

	"github.com/codepository/GoWebAnalytics/config"
	"github.com/codepository/GoWebAnalytics/model"
)

var conf = *config.Config

func waMain() error {

	// 启动数据库连接
	model.Setup()
	defer func() {
		log.Println("关闭数据库连接")
		model.GetDB().Close()
	}()
	// 启动redis连接
	model.SetRedis()
	defer func() {
		log.Println("关闭redis连接")
		if model.RedisOpen {
			model.RedisCli.Close()
		}
	}()
	// 连接管理器
	connmgr.New()
	defer func() {
		log.Println("优雅的关闭连接管理器")
		connmgr.CM.Stop()
	}()
	mux := router.Mux
	// 启动服务
	readTimeout, err := strconv.Atoi(conf.ReadTimeout)
	if err != nil {
		return err
	}
	writeTimeout, err := strconv.Atoi(conf.WriteTimeout)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:           fmt.Sprintf(":%s", conf.Port),
		Handler:        mux,
		ReadTimeout:    time.Duration(readTimeout * int(time.Second)),
		WriteTimeout:   time.Duration(writeTimeout * int(time.Second)),
		MaxHeaderBytes: 1 << 20,
	}
	// 监听关闭请求和关闭信号（Ctrl+C）
	interrupt := interruptListener(server)
	log.Printf("the application start up at port%s\n", server.Addr)
	if conf.TLSOpen == "true" {
		err = server.ListenAndServeTLS(conf.TLSCrt, conf.TLSKey)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil {
		log.Printf("Server err: %v", err)
		return err
	}
	<-interrupt
	return nil
}
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	debug.SetGCPercent(10)
	if err := waMain(); err != nil {
		os.Exit(1)
	}
}
