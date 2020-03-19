package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/codepository/GoWebAnalytics/service"
)

// Index 首页
func Index(writer http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(writer, "Hello world!")
}

// Test test
func Test(writer http.ResponseWriter, request *http.Request) {
	service.FlushBrowsings2DBFromRedis("2019-12-26")
}

// GetToken 获取token
func GetToken(request *http.Request) (string, error) {
	token := request.Header.Get("Authorization")
	if len(token) == 0 {
		request.ParseForm()
		if len(request.Form["token"]) == 0 {
			return "", errors.New("header Authorization 没有保存 token, url参数也不存在 token， 访问失败 ！")
		}
		token = request.Form["token"][0]
	}
	return token, nil

}
