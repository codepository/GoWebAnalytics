package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/codepository/GoWebAnalytics/connmgr"

	"github.com/codepository/GoWebAnalytics/config"
	"github.com/codepository/GoWebAnalytics/model"
	"github.com/codepository/GoWebAnalytics/router"
)

var conf = *config.Config

func main() {
	mux := router.Mux
	// 启动数据库连接
	model.Setup()
	// 启动redis连接
	model.SetRedis()
	// 连接管理器
	connmgr.New()
	// 启动服务
	readTimeout, err := strconv.Atoi(conf.ReadTimeout)
	if err != nil {
		panic(err)
	}
	writeTimeout, err := strconv.Atoi(conf.WriteTimeout)
	if err != nil {
		panic(err)
	}
	server := &http.Server{
		Addr:           fmt.Sprintf(":%s", conf.Port),
		Handler:        mux,
		ReadTimeout:    time.Duration(readTimeout * int(time.Second)),
		WriteTimeout:   time.Duration(writeTimeout * int(time.Second)),
		MaxHeaderBytes: 1 << 20,
	}
	log.Printf("the application start up at port%s", server.Addr)
	if conf.TLSOpen == "true" {
		err = server.ListenAndServeTLS(conf.TLSCrt, conf.TLSKey)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil {
		log.Printf("Server err: %v", err)
	}
}
