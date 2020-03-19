package model

import (
	"errors"
	"github.com/jinzhu/gorm"
)

// WebFlow 网页流量
type WebFlow struct {
	Model
	Domain   string `json:"domain"`   // 域名
	URL      string `json:"url"`      // 网址
	PV       int    `json:"pv"`       // 页面浏览量
	IP       int    `json:"ip"`       // 访问ip数
	UV       int    `json:"uv"`       // 独立访问者数
	Duration int    `json:"duration"` // 浏览时长
	Visits   int    `json:"visits"`   // 访问次数(半个小时内多次算一次)
	Bounce   int    `json:"bounce"`   // Bounce 只访问一次就跳出
	Date     string `json:"date"`     // 日期yyyy-mm-dd
}

// Save Save
func (w *WebFlow) Save() error {
	return db.Save(w).Error
}

// UpdateOrSave 如果存在就更新，否则就保存
func (w *WebFlow) UpdateOrSave() error {
	if len(w.URL) == 0 || len(w.Domain) == 0 {
		return errors.New("Webflow的url和domain不能为空")
	}
	fields := map[string]interface{}{
		"domain": w.Domain,
		"url":    w.URL,
		"date":   w.Date,
	}
	wf, err := findFirst(fields)

	if err == gorm.ErrRecordNotFound {
		return w.Save()
	}
	if err != nil {
		return err
	}
	wf.PV += w.PV
	wf.IP += w.IP
	wf.UV += w.UV
	wf.Duration += w.Duration
	wf.Visits += w.Visits
	wf.Bounce += w.Bounce
	return wf.Update()

}

// Update update
func (w *WebFlow) Update() error {
	return db.Model(w).Updates(w).Error
}

// findFirst 查找一第个
func findFirst(fields map[string]interface{}) (*WebFlow, error) {
	w := WebFlow{}
	err := db.Where(fields).First(&w).Error
	return &w, err
}

// FindTopContent FindTopContent
func FindTopContent(domain, start, end string) ([]*WebFlow, error) {
	return nil, nil
}
