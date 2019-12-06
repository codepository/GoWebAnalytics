package model

import "time"

// Browsing 用户访问习惯
type Browsing struct {
	UID        string    `json:"uid"`       // 用户id
	Depth      int       `json:"depth"`     // 访问页面数
	PV         int       `json:"pv"`        // 页面浏览量
	Visits     int       `json:"visits"`    // 访问次数(半个小时内多次算一次)
	Duration   uint64    `json:"duration"`  // 浏览时长
	Pageopend  int       `json:"pageopend"` // 同时打开页面数
	IP         string    `json:"ip"`
	Region     string    `json:"region"`     // 区域
	Platform   string    `json:"platform"`   // 操作系统
	Browser    string    `json:"browser"`    // 浏览器
	DeviceType int       `json:"devicetype"` // 终端类型 0为电脑、1为手机
	SR         string    `json:"sr"`         // 屏幕分辨率
	Start      time.Time `json:"start"`      // 开始时间
	NV         int       `json:"nv"`         // new visitor 0为用户回访,1为今天新访客
}
