package model

import (
	"github.com/jinzhu/gorm"
)

// RealtimeWebflow 实时网页流量
type RealtimeWebflow struct {
	Model
	Domain string `json:"domain"`
	PV     int    `json:"pv"`
	IP     int64  `json:"ip"`
	UV     int64  `json:"uv"`
	Date   string `json:"date"`
}

// Save 创建
func (r *RealtimeWebflow) Save() error {
	return db.Create(r).Error
}

// FindRealtimeDataByDomainAndStartDate 根据域名和开始日期查询实时网页流量
func FindRealtimeDataByDomainAndStartDate(domain, startDate string) ([]*RealtimeWebflow, error) {
	var data []*RealtimeWebflow
	err := db.Where("domain = ? AND date > ?", domain, startDate).Find(&data).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return data, nil
}
