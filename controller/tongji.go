package controller

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/codepository/GoWebAnalytics/service"

	"github.com/codepository/GoWebAnalytics/connmgr"
	"github.com/mumushuiding/util"
)

// WebData 获取页面流量信息
func WebData(writer http.ResponseWriter, request *http.Request) {
	var data connmgr.WebData
	err := util.Body2Struct(request, &data)
	if err != nil {
		fmt.Fprintln(writer, err)
	}
	connmgr.CM.NewWebData(&data)
}

// CloseWeb 关闭页面
func CloseWeb(writer http.ResponseWriter, request *http.Request) {
	var data connmgr.Duration
	err := util.Body2Struct(request, &data)
	if err != nil {
		fmt.Fprintln(writer, err)
	}
	data.Date = util.GetDateAsDefaultStr()
	// s, _ := util.ToJSONStr(data)
	// fmt.Println("closeweb:", s)
	connmgr.CM.CloseWeb(&data)
}

// GetRealtimeData 获取实时数据
func GetRealtimeData(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	var data service.RealtimeDataReq
	if len(request.Form["domain"]) > 0 {
		data.Domain = request.Form["domain"][0]
	}
	if len(request.Form["startDate"]) > 0 {
		data.StartDate = request.Form["startDate"][0]
	}
	// 身份验证
	service.CheckIdentity()
	// 获取域名
	result, err := service.GetRealtimeData(&data)
	if err != nil {
		fmt.Fprintln(writer, err)
	}
	fmt.Fprintln(writer, result)
}

// GetTopContent 获取url流量排名
func GetTopContent(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	req := getParams(request)
	if len(req.Domain) == 0 || len(req.StartDate) == 0 || len(req.EndDate) == 0 {
		fmt.Fprintln(writer, errors.New("domain 、 startDate、endDate 不能为空"))
		return
	}
	// 身份验证
	service.CheckIdentity()
	// 判断是否是当天
	req.StartDate = req.StartDate[0:10]
	req.EndDate = req.EndDate[0:10]
	if req.StartDate == req.EndDate && req.StartDate == time.Now().Format("2006-01-02") {
		result, err := service.GetTopContentFromRedis(req)
		if err != nil {
			fmt.Fprintln(writer, err)
		}
		fmt.Fprintln(writer, result)
	} else {
		result, err := service.GetTopContent(req)
		if err != nil {
			fmt.Fprintln(writer, err)
		}
		fmt.Fprintln(writer, result)
	}
}
func getParams(request *http.Request) *service.RealtimeDataReq {
	var data service.RealtimeDataReq
	if len(request.Form["domain"]) > 0 {
		data.Domain = request.Form["domain"][0]
	}
	if len(request.Form["startDate"]) > 0 {
		data.StartDate = request.Form["startDate"][0]
	}
	if len(request.Form["endDate"]) > 0 {
		data.EndDate = request.Form["endDate"][0]
	}
	return &data
}
