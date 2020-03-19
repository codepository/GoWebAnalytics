package service

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"strconv"
	"strings"
	"time"

	"github.com/mumushuiding/util"

	"github.com/codepository/GoWebAnalytics/model"
)

// WebData 页面信息
type WebData struct {
	Pageinfo model.Pageinfo `json:"p"`
	WebFlow  model.WebFlow  `json:"w"`
	Browsing model.Browsing `json:"b,omitempty"`
}

// RealtimeDataReq 实时数据获取请求
type RealtimeDataReq struct {
	Domain    string `json:"domain"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// IsNewVisitor 是否是新用户
func IsNewVisitor(domain, uid string) (bool, error) {
	return model.IsNewVisitor(domain, uid)
}

// GetRealtimeData 获取实时网页流量
func GetRealtimeData(req *RealtimeDataReq) (string, error) {
	if len(req.Domain) == 0 || len(req.StartDate) == 0 {
		return "", errors.New("domain 和 startDate 不能为空")
	}
	datas, err := model.FindRealtimeDataByDomainAndStartDate(req.Domain, req.StartDate)
	if err != nil {
		return "", err
	}
	r, err := util.ToJSONStr(datas)
	if err != nil {
		return "", err
	}
	return r, nil
}

// GetTopContent 获取url流量排名
func GetTopContent(req *RealtimeDataReq) (string, error) {
	datas, err := model.FindTopContent(req.Domain, req.StartDate, req.EndDate)
	if err != nil {
		return "", err
	}
	r, err := util.ToJSONStr(datas)
	if err != nil {
		return "", err
	}
	return r, nil
}

// GetTopContentFromRedis 从redis获取当日流量排名
func GetTopContentFromRedis(req *RealtimeDataReq) (string, error) {
	sort := &redis.Sort{}
	sort.By = GetRedisWebflowKey(req.Domain, req.StartDate, "*") + "->PV"
	sort.Order = "desc"
	sort.Offset = 0
	sort.Count = 100
	sort.Get = []string{GetRedisPageinfoKey(req.StartDate, "*")}
	r := model.RedisCli.Sort(GetRedisURLKey(req.StartDate), sort)
	if r.Err() != nil {
		return "", r.Err()
	}
	var result []WebData
	fmt.Println("page:", r.Val())
	for _, val := range r.Val() {
		page := model.Pageinfo{}
		util.Str2Struct(val, &page)
		webflow, err := getWebflowFromRedis(GetRedisWebflowKey(req.Domain, req.StartDate, page.URL))
		if err != nil {
			return "", err
		}
		var webdata WebData
		webdata.Pageinfo = page
		webdata.WebFlow = *webflow
		result = append(result, webdata)
	}
	s, _ := util.ToJSONStr(result)
	return s, nil
}
func getWebflowFromRedis(key string) (*model.WebFlow, error) {
	var webflow model.WebFlow
	r := model.RedisCli.HMGet(key, "PV", "IP", "UV", "Visits", "Duration")
	if r.Err() != nil && r.Err() != redis.Nil {
		return nil, r.Err()
	}
	// s2, _ := util.ToJSONStr(webflow)
	// log.Printf("webflow-before-val:%s\n", s2)
	if r.Val()[0] != nil {
		// s, _ := util.ToJSONStr(r.Val())
		// log.Printf("hmget-key:%s,val:%s\n", key, s)
		var vals []int
		for _, v := range r.Val() {
			switch v.(type) {
			case string:
				x, _ := strconv.Atoi(v.(string))
				vals = append(vals, x)
			case int:
				vals = append(vals, v.(int))
			}
		}
		// log.Println("vals", vals)
		webflow.PV += vals[0]
		webflow.IP += vals[1]
		webflow.UV += vals[2]
		webflow.Visits += vals[3]
		webflow.Duration += vals[4]
	}
	return &webflow, nil
}

// FlushBrowsings2DBFromRedis 将redis中保存的用户浏览习惯保存到数据库
func FlushBrowsings2DBFromRedis(date string) {
	uidkey := GetRedisUIDKey(date)
	for {
		// 判断集合是否为空
		s := model.RedisCli.SCard(uidkey)
		if s.Err() != nil {
			Log(s.Err())
			break
		}
		// 集合为空就删除键值
		if s.Val() <= 0 {
			if err := model.RedisCli.Del(uidkey).Err(); err != nil {
				Log(err)
			}
			break
		}
		// pop uid
		sp := model.RedisCli.SPopN(uidkey, 500)
		if sp.Err() != nil {
			Log(sp.Err())
			break
		}
		uids := sp.Val()
		pipe := model.RedisCli.Pipeline()
		for _, uid := range uids {
			// 获取用户browsing
			browsings, err := GetBrowsingsByUIDFromRedis(date, uid)
			if err != nil {
				Log(err)
				continue
			}
			for _, browsing := range browsings {
				browsing.UID = uid
				browsing.Date = date
				// 存储到数据库
				if err := browsing.UpdateOrSave(); err != nil {
					Log(err)
					continue
				}
			}
			// 删除redis中的browsing
			pipe.Del(GetRedisBrowsingKey(date, uid))
		}
		pipe.Exec()
	}
}

// GetBrowsingsByUIDFromRedis 获取用户在所有域名的访问习惯
func GetBrowsingsByUIDFromRedis(date, uid string) ([]model.Browsing, error) {
	key := GetRedisBrowsingKey(date, uid)
	r := model.RedisCli.HGetAll(key)
	if r.Err() != nil && r.Err() != redis.Nil {
		return nil, r.Err()
	}
	result := []model.Browsing{}
	for domain, str := range r.Val() {
		b := model.Browsing{}
		util.Str2Struct(str, &b)
		b.Domain = domain
		result = append(result, b)
	}
	return result, nil
}

// GetBrowsingFromRedis 根据键值从redis获取值
func GetBrowsingFromRedis(key, domain string) (model.Browsing, error) {
	browsing := model.Browsing{}
	r := model.RedisCli.HGet(key, domain)
	if r.Err() != nil && r.Err() != redis.Nil {
		return browsing, r.Err()
	}
	if len(r.Val()) > 0 {
		if err := util.Str2Struct(r.Val(), &browsing); err != nil {
			return browsing, err
		}
	}
	// if r.Val()[0] != nil {
	// 	var vals []int
	// 	for _, v := range r.Val() {
	// 		switch v.(type) {
	// 		case string:
	// 			x, _ := strconv.Atoi(v.(string))
	// 			vals = append(vals, x)
	// 		case int:
	// 			vals = append(vals, v.(int))
	// 		}
	// 	}
	// 	browsing.Depth += vals[0]
	// 	browsing.PV += vals[1]
	// 	browsing.Visits += vals[2]
	// 	browsing.Duration += vals[3]
	// }
	return browsing, nil

}

// FlushWebflow2DBFromRedis 将redis中保存的网页流量保存到数据库
func FlushWebflow2DBFromRedis(date string) {
	urlkey := GetRedisURLKey(date)
	for {
		// 判断是否为空
		s := model.RedisCli.SCard(urlkey)
		if s.Err() != nil {
			Log(s.Err())
			break
		}
		// 删除键值
		if s.Val() <= 0 {
			if err := model.RedisCli.Del(urlkey).Err(); err != nil {
				Log(err)
			}
			break
		}
		// pop Url
		sp := model.RedisCli.SPopN(urlkey, 500)
		if sp.Err() != nil {
			Log(sp.Err())
			break
		}
		urls := sp.Val()
		pipe := model.RedisCli.Pipeline()
		for _, url := range urls {
			// 获取webflow值
			domain := getDomainFromURL(url)
			wkey := GetRedisWebflowKey(domain, date, url)
			webfow, err := getWebflowFromRedis(wkey)
			if err != nil {
				Log(err)
				continue
			}
			// 存储到数据库
			webfow.Domain = domain
			webfow.Date = date
			webfow.URL = url
			if err := webfow.UpdateOrSave(); err != nil {
				Log(err)
				continue
			}
			pipe.Del(wkey)
		}
		// 批量删除reddis中存储的webflow
		pipe.Exec()
	}
}

// getDomainFromURL 从url获取domain
func getDomainFromURL(url string) string {
	a1 := strings.Split(url, "//")[1]
	a2 := strings.Split(a1, "/")[0]
	a3 := strings.Split(a2, ":")[0]
	return a3
}

// GetRegistryDomains 获取所有注册的域名
func GetRegistryDomains() ([]*model.Domainmgr, error) {
	return model.GetAllRegistryDomains()
}

// SaveRealtimeWebflow 存储网页流量
func SaveRealtimeWebflow(data *model.RealtimeWebflow) error {
	return data.Save()
}

// AddUID2Redis 将uid存储到redis
func AddUID2Redis(date, uid string) error {
	key := GetRedisUIDKey(date)
	r := model.RedisCli.SIsMember(key, uid)
	if r.Err() != nil {
		Log(r.Err())
		return r.Err()
	}
	if !r.Val() {
		return model.RedisCli.SAdd(key, uid).Err()
	}
	return nil
}

// RedisKeyWithTongjiAboutTodayExpireAtTomorrow 关于tongji的key在明日凌晨过期
func RedisKeyWithTongjiAboutTodayExpireAtTomorrow() {
	tm := time.Now()
	date := util.FormatDate(tm, util.YYYY_MM_DD)
	tm = tm.Add(time.Hour * 24)
	// tongji_uid_<yyyy-mm-dd>
	model.RedisCli.ExpireAt(GetRedisUIDKey(date), tm)
	// tongji_url_<yyyy-mm-dd>
	model.RedisCli.ExpireAt(GetRedisURLKey(date), tm)

}

// GetRedisUIDKey tongji_uid_<yyyy-mm-dd> 保存今日访问用户的uid
func GetRedisUIDKey(date string) string {
	return fmt.Sprintf("tongji_uid_%s", date)
}

// GetRedisURLKey tongji_url_<yyyy-mm-dd>获取在redis中保存url集合的key
func GetRedisURLKey(defaultdate string) string {
	return fmt.Sprintf("tongji_url_%s", defaultdate)
}

// GetRedisPageinfoKey tongji_pageinfo_<yyyy-mm-dd>_<url> 纪录页面信息的key
func GetRedisPageinfoKey(defaultdate, url string) string {
	return fmt.Sprintf("tongji_pageinfo_%s_%s", defaultdate, url)
}

// GetRedisIPKey tongji_ip_<yyyy-mm-dd>_<ip> 统计指定ip访问过的页面的key
func GetRedisIPKey(defaultdate, ip string) string {
	return fmt.Sprintf("tongji_ip_%s_%s", defaultdate, ip)
}

// GetRedisWebflowKey tongji_webflow_<domain>_<yyyy-mm-dd>-<url> 统计单个页面流量的key
func GetRedisWebflowKey(domain, defaultdate, url string) string {
	return fmt.Sprintf("tongji_webflow_%s_%s_%s", domain, defaultdate, url)
}

// GetRedisVisitorKey tongji_visitor_url_<yyyy-mm-dd>_<visitor> 统计单个用户查看过的页面的key
func GetRedisVisitorKey(defaultdate, uid string) string {
	return fmt.Sprintf("tongji_visitor_url_%s_%s", defaultdate, uid)
}

// GetRedisBrowsingKey tongji_browsing_<yyyy-mm-dd>_<visitor> 统计某个用户的浏览习惯的key
func GetRedisBrowsingKey(defaultdate, uid string) string {
	return fmt.Sprintf("tongji_browsing_%s_%s", defaultdate, uid)
}

// GetRedisVisitNumbersKey tongji_visitnumbers_url_<yyyy-mm-dd>_<visitor> 统计用户访问次数的key
func GetRedisVisitNumbersKey(defaultdate, uid string) string {
	return fmt.Sprintf("tongji_visitnumbers_url_%s_%s", defaultdate, uid)
}

// GetredisNewVisitorKey tongji_newvisitor_<yyyy-mm-dd>_<domain> 统计某个域名今日新用户的key
func GetredisNewVisitorKey(defaultdate, domain string) string {
	return fmt.Sprintf("tongji_newvisitor_%s_%s", defaultdate, domain)
}

// GetRedisTimePVKey tongji_time_<domain>_pv 统计某个域名实时打开页面数
func GetRedisTimePVKey(domain string) string {
	return fmt.Sprintf("tongji_time_%s_pv", domain)
}

// GetRedisTimeIPKey tongji_time_<domain>_ip 统计某个域名实时在线IP数
func GetRedisTimeIPKey(domain string) string {
	return fmt.Sprintf("tongji_time_%s_ip", domain)
}

// GetRedisTimeUVKey tongji_time_<domain>_uv 统计某个域名实时在线独立用户数
func GetRedisTimeUVKey(domain string) string {
	return fmt.Sprintf("tongji_time_%s_uv", domain)
}
