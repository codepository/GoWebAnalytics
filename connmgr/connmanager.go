package connmgr

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis"

	"github.com/mumushuiding/util"

	"github.com/codepository/GoWebAnalytics/model"
)

const maxFailedAttempts = 3

// CM 连接管理器
var CM *ConnManager

const handlePerTime = 500

// ConnManager 连接管理器
type ConnManager struct {
	cfg           *Config
	start         int32
	stop          int32
	connReqCount  uint64
	requests      chan interface{}
	pageinfos     map[string]*model.Pageinfo
	pageinfoLock  sync.RWMutex
	webflows      map[string]*model.WebFlow //key为url+date
	webflowsLock  sync.RWMutex
	browsings     map[string]*model.Browsing //key为url+date
	browsingsLock sync.RWMutex
	quit          chan struct{}
}

// Config Config
type Config struct {
	OnWebData func(*WebData)
	// HandleWebFlow 统计流量
	HandleWebFlow func(*webFlowReq)
	// HandleBrowsing 纪录用户习惯
	HandleBrowsing func(*model.Browsing)
}

// WebData 页面信息
type WebData struct {
	Pageinfo model.Pageinfo `json:"p"`
	WebFlow  model.WebFlow  `json:"w"`
	Browsing model.Browsing `json:"b"`
	Type     string         `json:"t"`
}
type webFlowReq struct {
	webflow  model.WebFlow
	browsing model.Browsing
}

// Start 连接管理器初始化
func (cm *ConnManager) Start() {
	// Already started?
	if atomic.AddInt32(&cm.start, 1) != 1 {
		return
	}
	fmt.Println("启动连接管理器")
	go cm.connHandler()
}

// Stop gracefully shuts down the connection manager.
func (cm *ConnManager) Stop() {
	if atomic.AddInt32(&cm.stop, 1) != 1 {
		log.Println("连接管理器已经停止")
		return
	}
	// 将map中缓存的信息缓存到redis
	close(cm.quit)
	close(cm.requests)
	log.Println("连接管理器停止")
}

// New 返回一个连接管理器
func New() {
	cm := ConnManager{
		requests:  make(chan interface{}, 10),
		quit:      make(chan struct{}),
		pageinfos: make(map[string]*model.Pageinfo),
		webflows:  make(map[string]*model.WebFlow),
		browsings: make(map[string]*model.Browsing),
	}
	cfg := &Config{
		OnWebData:     cm.inWebData,
		HandleWebFlow: cm.handleWebFlow,
	}
	cm.cfg = cfg
	CM = &cm
	CM.Start()
}

// connHandler handle all connection related request
func (cm *ConnManager) connHandler() {
out:
	for {
		select {
		case req := <-cm.requests:
			switch msg := req.(type) {
			case *WebData:
				go cm.cfg.OnWebData(msg)
			case webFlowReq:
				go cm.cfg.HandleWebFlow(&msg)
			}
		case <-cm.quit:
			break out
		}
	}
	fmt.Println("退出连接管理器")
}

// NewWebData 新建连接
func (cm *ConnManager) NewWebData(w *WebData) {
	select {
	case cm.requests <- w:
		atomic.AddUint64(&cm.connReqCount, 1)
	case <-cm.quit:
		return
	}
}
func (cm *ConnManager) inWebData(w *WebData) {
	// 判断url地址是否已经存在
	if !model.RedisCli.SIsMember(getRedisURLKey(util.GetDateAsDefaultStr()), w.Pageinfo.URL).Val() {
		go cm.addPageinfo(w)
	}
	w.WebFlow.Date = util.GetDateAsDefaultStr()
	f := webFlowReq{
		webflow:  w.WebFlow,
		browsing: w.Browsing,
	}
	cm.requests <- f
}
func (cm *ConnManager) log(err error) {
	fmt.Println(err)
}

// getRedisURLKey 获取在redis中保存url集合的key
func getRedisURLKey(defaultdate string) string {
	return fmt.Sprintf("tongji_url_%s", defaultdate)
}
func getRedisPageinfoKey(defaultdate, url string) string {
	return fmt.Sprintf("tongji_pageinfo_%s_%s", defaultdate, url)
}
func getRedisIPKey(defaultdate, ip string) string {
	return fmt.Sprintf("tongji_ip_%s_%s", defaultdate, ip)
}
func getRedisWebflowKey(defaultdate, url string) string {
	return fmt.Sprintf("tongji_webflow_%s_%s", defaultdate, url)
}
func getRedisBrowsingKey(defaultdate, uid string) string {
	return fmt.Sprintf("tongji_browsing_%s_%s", defaultdate, uid)
}
func (cm *ConnManager) addPageinfo(w *WebData) {
	cm.pageinfos[w.Pageinfo.URL] = &w.Pageinfo
	if len(cm.pageinfos) > 1 {
		go cm.handlePageinfo()
	}
}

// handleWebFlow 统计网络流量
func (cm *ConnManager) handleWebFlow(req *webFlowReq) {
	// pv
	req.webflow.PV++
	req.browsing.PV++
	// ip今天是否已经访问过了,
	if len(req.browsing.IP) > 0 && !isIPVisited(req.browsing.IP, req.webflow.URL) {
		req.webflow.IP++
	}
	if len(req.browsing.UID) > 0 {
		// uv
		if !isUVVisitedToday(req.browsing.UID, req.webflow.URL) {
			req.webflow.UV++
			req.browsing.Depth++
		}
		// visits
		if !isUVVisited(req.browsing.UID, req.webflow.URL) {
			req.webflow.Visits++
			req.browsing.Visits++
		}
		// NV 是否为新客户,默认为旧
		if isNewVisitor(req.browsing.UID) {
			req.browsing.NV = 1
		}
	}
	// Pageopend

	// 添加webflow到map
	go cm.addWebflow(&req.webflow)
	// 添加browsing 到 map
	go cm.addBrowsing(&req.browsing)
}
func isIPVisited(ip, url string) bool {
	// 添加锁
	// 判断是否已经存在,需要加分布式锁
	// 解锁
	return false
}
func isUVVisitedToday(uid, url string) bool {
	return false
}
func isUVVisited(uid, url string) bool {
	return false
}
func isNewVisitor(uid string) bool {
	return false
}

// addWebflow 添加webflow
func (cm *ConnManager) addWebflow(data *model.WebFlow) {
	// 锁表
	cm.webflowsLock.Lock()
	// 判断是否存在,不存在就添加,否则
	w := cm.webflows[data.URL+data.Date]
	if w != nil {
		fmt.Println("webflow 已经存在")
		w.PV += data.PV
		w.IP += data.IP
		w.UV += data.UV
		w.Visits += data.Visits
	} else {
		fmt.Println("webflow 不存在")
		w = data
	}
	// 释放表
	cm.webflowsLock.Unlock()
	if len(cm.webflows) > handlePerTime {
		cm.flushWebflowsToRedis()
	}
}

// addBrowsing 添加browsing
func (cm *ConnManager) addBrowsing(data *model.Browsing) {
	cm.browsingsLock.Lock()
	b := cm.browsings[data.UID+util.FormatDate(data.Start, util.YYYY_MM_DD)]
	if b != nil {
		fmt.Println("browsing 已经存在")
		b.Depth += data.Depth
		b.PV += data.PV
		b.Visits += data.Visits
		b.Pageopend += data.Pageopend
	} else {
		fmt.Println("browsing 不存在")
		b = data
	}
	cm.browsingsLock.Unlock()
	if len(cm.browsings) > handlePerTime {
		cm.flushBrowsingsToRedis()
	}
}

// flushWebflowToRedis 流量累加到redis
func (cm *ConnManager) flushWebflowsToRedis() {
	// 加锁
	cm.webflowsLock.Lock()
	// 提取需要处理的数据
	result := make(map[string]*model.WebFlow)
	i := 0
	for k, v := range cm.webflows {
		i++
		x := new(model.WebFlow)
		*x = *v
		result[v.URL] = x
		delete(cm.webflows, k)
		if i >= handlePerTime {
			break
		}
	}
	// 解锁
	cm.webflowsLock.Unlock()
	for _, webflow := range result {
		go cm.freshWebflow(webflow)
	}
}
func (cm *ConnManager) freshWebflow(webflow *model.WebFlow) {
	// 保存原值，当失败时可以返回
	temp := new(model.WebFlow)
	*temp = *webflow
	key := getRedisWebflowKey(webflow.Date, webflow.URL)
	// 要处理的事务
	txf := func(tx *redis.Tx) error {
		// 获取并更新值
		r := tx.HMGet(key, "PV", "IP", "UV", "Visits", "Bounce")
		if r.Err() != nil && r.Err() != redis.Nil {
			return r.Err()
		}
		vals := r.Val()
		fmt.Println(vals)
		webflow.PV += vals[0].(int)
		webflow.IP += vals[1].(int)
		webflow.UV += vals[2].(int)
		webflow.Visits += vals[3].(int)
		webflow.Bounce += vals[4].(int)
		fmt.Println(webflow)
		// 存储到redis
		_, err := tx.Pipelined(func(pipe redis.Pipeliner) error {
			fields := map[string]interface{}{
				"PV":     webflow.PV,
				"IP":     webflow.IP,
				"UV":     webflow.UV,
				"Visits": webflow.Visits,
				"Bounce": webflow.Bounce,
			}
			return pipe.HMSet(key, fields).Err()
		})
		return err
	}
	// 监测锁
	for {
		i, j := 0, 0
		for { // 每隔1秒连续尝试100次
			i++
			if i > 100 { // 连续获取100次锁失败之后，隔1秒再获取
				time.Sleep(1 * time.Second)
				j++
				if j > 10 {
					cm.addWebflow(webflow)
					break
				}
			}
			i = 0
		}
		err := model.RedisCli.Watch(txf, key)
		if err != redis.TxFailedErr {
			cm.log(err)
			// 将失败重新存入map
			cm.addWebflow(temp)
		}
	}
}

// 用户浏览情况累加到redis
func (cm *ConnManager) flushBrowsingsToRedis() {
	// 加锁
	cm.browsingsLock.Lock()
	// 提取需要处理的数据
	result := make(map[string]*model.Browsing)
	i := 0
	for k, v := range cm.browsings {
		i++
		x := new(model.Browsing)
		*x = *v
		result[v.UID] = x
		delete(cm.browsings, k)
		if i >= handlePerTime {
			break
		}
	}
	// 解锁
	cm.browsingsLock.Unlock()
	for _, browsing := range result {
		go cm.freshBrowsing(browsing)
	}
}
func (cm *ConnManager) freshBrowsing(browsing *model.Browsing) {
	// 保存原值，当失败时可以返回
	temp := new(model.Browsing)
	*temp = *browsing
	key := getRedisBrowsingKey(util.FormatDate(browsing.Start, util.YYYY_MM_DD), browsing.UID)
	// 要处理的事务
	txf := func(tx *redis.Tx) error {
		// 获取并更新值
		r := tx.HMGet(key, "Depth", "PV", "Visits", "Duration")
		if r.Err() != nil && r.Err() != redis.Nil {
			return r.Err()
		}
		vals := r.Val()
		fmt.Println(vals)
		browsing.Depth += vals[0].(int)
		browsing.PV += vals[1].(int)
		browsing.Visits += vals[2].(int)
		browsing.Duration += vals[3].(uint64)
		fmt.Println(browsing)
		// 存储到redis
		_, err := tx.Pipelined(func(pipe redis.Pipeliner) error {
			fields := map[string]interface{}{
				"Depth":    browsing.Depth,
				"PV":       browsing.PV,
				"Visits":   browsing.Visits,
				"Duration": browsing.Duration,
			}
			return pipe.HMSet(key, fields).Err()
		})
		return err
	}
	// 监测锁
	for {
		i, j := 0, 0
		for { // 每隔1秒连续尝试100次
			i++
			if i > 100 { // 连续获取100次锁失败之后，隔1秒再获取
				time.Sleep(1 * time.Second)
				j++
				if j > 10 {
					cm.addBrowsing(browsing)
					break
				}
			}
			i = 0
		}
		err := model.RedisCli.Watch(txf, key)
		if err != redis.TxFailedErr {
			cm.log(err)
			// 将失败重新存入map
			cm.addBrowsing(temp)
		}
	}
}

// handlePageinfo 保存page到redis，每次批处理500个
func (cm *ConnManager) handlePageinfo() {
	var result = make(map[string]*model.Pageinfo)
	cm.pageinfoLock.Lock()
	for url, page := range cm.pageinfos {
		var x = new(model.Pageinfo)
		*x = *page
		result[url] = x
		delete(cm.pageinfos, url)
	}
	cm.pageinfoLock.Unlock()
	defaultdate := util.GetDateAsDefaultStr()
	urlkey := getRedisURLKey(defaultdate)
	pipe := model.RedisCli.Pipeline()
	for url, page := range result {
		pipe.SAdd(urlkey, url)
		pageinfokey := getRedisPageinfoKey(defaultdate, url)
		s, _ := util.ToJSONStr(page)
		fmt.Println(pageinfokey)
		pipe.SAdd(pageinfokey, s)
	}
	_, err := pipe.Exec()
	if err != nil {
		cm.log(err)
		for url, page := range result {
			cm.pageinfos[url] = page
		}
	}
}
