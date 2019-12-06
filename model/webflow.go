package model

// WebFlow 网页流量
type WebFlow struct {
	Model
	DM     string `json:"dm"`     // 域名
	URL    string `json:"url"`    // 网址
	PV     int    `json:"pv"`     // 页面浏览量
	IP     int    `json:"ip"`     // 访问ip数
	UV     int    `json:"uv"`     // 独立访问者数
	Visits int    `json:"visits"` // 访问次数(半个小时内多次算一次)
	Bounce int    `json:"bounce"` // Bounce 只访问一次就跳出
	Date   string `json:"date"`   // 日期yyyy-mm-dd
}
