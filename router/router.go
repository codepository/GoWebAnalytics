package router

import (
	"net/http"

	"github.com/codepository/GoWebAnalytics/config"
	"github.com/codepository/GoWebAnalytics/controller"
)

// Mux 路由
var Mux = http.NewServeMux()
var conf = *config.Config

func interceptor(h http.HandlerFunc) http.HandlerFunc {
	return crossOrigin(h)
}
func crossOrigin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", conf.AccessControlAllowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", conf.AccessControlAllowMethods)
		w.Header().Set("Access-Control-Allow-Headers", conf.AccessControlAllowHeaders)
		h(w, r)
	}
}
func init() {
	setMux()
}
func setMux() {
	Mux.HandleFunc("/api/v1/test/test", controller.Test)
	Mux.HandleFunc("/api/v1/test/index", interceptor(controller.Index))
	Mux.HandleFunc("/api/v1/tongji/webdata", interceptor(controller.WebData))
	Mux.HandleFunc("/api/v1/tongji/close", interceptor(controller.CloseWeb))
	Mux.HandleFunc("/api/v1/tongji/getRealtimeData", interceptor(controller.GetRealtimeData))
	Mux.HandleFunc("/api/v1/tongji/getTopContent", interceptor(controller.GetTopContent))
}
