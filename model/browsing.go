package model

import (
	"errors"
	"github.com/jinzhu/gorm"
)

// Browsing 用户访问习惯
type Browsing struct {
	UID        string `gorm:"primary_key" json:"uid"` // 用户id
	Domain     string `json:"domain"`                 // 域名
	Depth      int    `json:"depth"`                  // 访问页面数
	PV         int    `json:"pv"`                     // 页面浏览量
	Visits     int    `json:"visits"`                 // 访问次数(半个小时内多次算一次)
	Duration   int    `json:"duration"`               // 浏览时长
	Pageopend  int    `json:"pageopend"`              // 同时打开页面数
	IP         string `json:"ip"`
	Region     string `json:"region"`     // 区域,未考虑一天内出现在多地的情况
	Platform   string `json:"platform"`   // 操作系统
	Browser    string `json:"browser"`    // 浏览器
	DeviceType int    `json:"devicetype"` // 终端类型 0为电脑、1为手机
	SR         string `json:"sr"`         // 屏幕分辨率
	NV         int    `json:"nv"`         // new visitor 0为用户回访,1为今天新访客
	Date       string `json:"date"`       // 浏览日期
}

// IsNewVisitor 是否是新用户
func IsNewVisitor(domain, uid string) (bool, error) {
	var b Browsing
	err := db.Select("uid").Find(&b).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

// Save save
func (b *Browsing) Save() error {
	return db.Save(b).Error
}

// UpdateOrSave 存在就更新否则就保存
func (b *Browsing) UpdateOrSave() error {
	if len(b.UID) == 0 || len(b.Domain) == 0 || len(b.Date) == 0 {
		return errors.New("Browsing的uid、date和domain不能为空")
	}
	fields := map[string]interface{}{
		"uid":    b.UID,
		"domain": b.Domain,
		"date":   b.Date,
	}
	old, err := findFirstBrowsing(fields)
	if err == gorm.ErrRecordNotFound {
		print("未找到纪录")
		return b.Save()
	}
	if err != nil {
		return err
	}
	old.Duration += b.Duration
	old.IP += b.IP
	old.PV += b.PV
	old.Pageopend += b.Pageopend
	old.Visits += b.Visits
	old.Depth += b.Depth
	return db.Model(old).Updates(old).Error
}

// findFirstBrowsing 返回第一条纪录
func findFirstBrowsing(fields map[string]interface{}) (*Browsing, error) {
	b := Browsing{}
	err := db.Where(fields).First(&b).Error
	return &b, err
}
