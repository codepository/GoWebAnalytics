package model

// Domainmgr 域名管理
type Domainmgr struct {
	Model
	Domain string `json:"domain"`
}

// Save save
func (d *Domainmgr) Save() error {
	return db.Create(d).Error
}

// GetAllRegistryDomains 获取所有注册的域名
func GetAllRegistryDomains() ([]*Domainmgr, error) {
	var data []*Domainmgr
	err := db.Find(&data).Error
	if err != nil {
		return nil, err
	}
	return data, nil
}
