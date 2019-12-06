package service

import (
	"fmt"

	"github.com/codepository/GoWebAnalytics/model"
)

// WebData 页面信息
type WebData struct {
	Pageinfo model.Pageinfo `json:"p"`
	WebFlow  model.WebFlow  `json:"w"`
	Browsing model.Browsing `json:"b"`
	Type     string         `json:"t"`
}

// Tongji 纪录访问信息
func Tongji(data map[string]interface{}) {
	fmt.Println(data["p"]["url"])
	if len(data["p"]["url"]) > 0 {
		isURLExists(&data)
	}
}
