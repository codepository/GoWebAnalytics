package connmgr

import (
	"errors"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codepository/GoWebAnalytics/service"

	"github.com/go-redis/redis"

	"github.com/mumushuiding/util"

	"github.com/codepository/GoWebAnalytics/model"
)

const maxFailedAttempts = 3

// CM 连接管理器
var CM *ConnManager

// handlePerTime 当达到指定数时，批量处理
const handlePerTime = 500

// 每隔指定时间将缓存保存到redis
const flushCacheToRedisPeriod = 10

// 每隔指定时间从redis读取域名流量信息
const getRealtimeWebflowPeriod = 300

// ConnManager 连接管理器
type ConnManager struct {
	cfg          *Config
	start        int32
	stop         int32
	connReqCount uint64
	requests     chan interface{}
	// pageinfos                map[string]*model.Pageinfo
	// pageinfoLock             sync.RWMutex
	webflows                 map[string]*model.WebFlow //key为url+date
	webflowsLock             sync.RWMutex
	browsings                map[string]*model.Browsing //key为url+date
	browsingsLock            sync.RWMutex
	pvrealtime               map[string]int64 // 实时打开页面数
	pvlock                   sync.RWMutex
	iprealtime               map[string]map[string]interface{} // ip实时打开页面数
	iplock                   sync.RWMutex
	uvrealtime               map[string]map[string]interface{} // uv实时打开页面数
	uvlock                   sync.RWMutex
	quit                     chan struct{}
	flushcacheTicker         *time.Ticker
	getRealtimeWebflowTicker *time.Ticker
}

// Config Config
type Config struct {
	OnWebData func(*WebData)
	// HandleWebFlow 统计流量
	HandleWebFlow func(*webFlowReq)
	// HandleBrowsing 纪录用户习惯
	HandleBrowsing func(*model.Browsing)
	// HandleDuration 浏览时长
	HandleDuration func(*Duration)
}

// WebData 页面信息
type WebData struct {
	Pageinfo model.Pageinfo `json:"p"`
	WebFlow  model.WebFlow  `json:"w"`
	Browsing model.Browsing `json:"b"`
	Type     string         `json:"t"`
}
type webFlowReq struct {
	webflow  *model.WebFlow
	browsing *model.Browsing
}

// Duration 网页浏览时长
type Duration struct {
	Domain   string `json:"domain"`
	Duration int    `json:"duration"`
	IP       string `json:"ip"`
	UID      string `json:"uid"`
	URL      string `json:"url"`
	Date     string `json:"date"`
}

// Start 连接管理器初始化
func (cm *ConnManager) Start() {
	// Already started?
	if atomic.AddInt32(&cm.start, 1) != 1 {
		return
	}
	log.Println("启动连接管理器")
	go cm.connHandler()
	go func() {
	out:
		for {
			select {
			case <-cm.flushcacheTicker.C:
				// 网页流量保存到redis
				go cm.flushWebflowsToRedis()
				// 用户访问信息保存到redis
				go cm.flushBrowsingsToRedis()
				// 将实时PV、IP、UV保存到redis
				go cm.flushRealtimeDataToRedis()
			case <-cm.quit:
				break out
			}
		}
	}()
	go func() {
	out:
		for {
			// 保存实时流量
			select {
			case <-cm.getRealtimeWebflowTicker.C:
				go cm.persistRealtimeWebflow()
			case <-cm.quit:
				break out
			}
		}
	}()
	// 关联文件在多个文件夹无法测试，用以下代码进行测试
	// go func() {
	// 	println("this is a test func,you must delete it later!!!")
	// 	n := time.Now()
	// 	date := util.FormatDate(n, util.YYYY_MM_DD)
	// 	service.FlushWebflow2DBFromRedis(date)
	// }()
	// 测试用户习惯到mysql
	// go func() {
	// 	println("this is a test func,you must delete it later!!!")
	// 	n := time.Now()
	// 	date := util.FormatDate(n, util.YYYY_MM_DD)
	// 	println(service.GetRedisKeyMatchTongjiWithDate(date))

	// 	service.FlushBrowsings2DBFromRedis(date)
	// }()
	// 每天0点保存数据到数据库
	go func() {
	out:
		for {
			now := time.Now()
			next := now.Add(time.Hour * 24)
			next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
			t := time.NewTimer(next.Sub(now))
			select {
			case <-cm.quit:
				break out
			case <-t.C:
				// 保存网页流量到数据库
				n := time.Now()
				n = n.Add(time.Hour * -24)
				date := util.FormatDate(n, util.YYYY_MM_DD)
				service.FlushWebflow2DBFromRedis(date)
				// 保存用户习惯到数据库
				service.FlushBrowsings2DBFromRedis(date)
				// 设置 key 过期时间
				service.RedisKeyWithTongjiAboutTodayExpireAtTomorrow()
			}
		}
	}()
}

// Stop gracefully shuts down the connection manager.
func (cm *ConnManager) Stop() {
	if atomic.AddInt32(&cm.stop, 1) != 1 {
		log.Println("连接管理器已经关闭")
		return
	}
	// 将map中缓存的信息缓存到redis
	close(cm.quit)
	close(cm.requests)
	// 暂停定时器
	cm.flushcacheTicker.Stop()
	cm.getRealtimeWebflowTicker.Stop()
	log.Println("连接管理器关闭成功")
}

// New 返回一个连接管理器
func New() {
	cm := ConnManager{
		requests: make(chan interface{}, 10),
		quit:     make(chan struct{}),
		// pageinfos:                make(map[string]*model.Pageinfo),
		webflows:                 make(map[string]*model.WebFlow),
		browsings:                make(map[string]*model.Browsing),
		pvrealtime:               make(map[string]int64),
		iprealtime:               make(map[string]map[string]interface{}),
		uvrealtime:               make(map[string]map[string]interface{}),
		flushcacheTicker:         time.NewTicker(time.Second * flushCacheToRedisPeriod),
		getRealtimeWebflowTicker: time.NewTicker(time.Second * getRealtimeWebflowPeriod),
	}
	cfg := &Config{
		OnWebData:      cm.inWebData,
		HandleWebFlow:  cm.handleWebFlow,
		HandleDuration: cm.handleDuration,
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
			case *webFlowReq:
				go cm.cfg.HandleWebFlow(msg)
			case *Duration:
				go cm.cfg.HandleDuration(msg)
			}
		case <-cm.quit:
			break out
		}
	}
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

// CloseWeb 关闭网页
func (cm *ConnManager) CloseWeb(d *Duration) {
	select {
	case cm.requests <- d:
		atomic.AddUint64(&cm.connReqCount, 1)
	case <-cm.quit:
		return
	}
}
func (cm *ConnManager) inWebData(w *WebData) {
	// 判断url地址是否已经存在
	w.Pageinfo.Dm = w.Browsing.Domain
	date := util.GetDateAsDefaultStr()
	if !model.RedisCli.SIsMember(service.GetRedisURLKey(date), w.Pageinfo.URL).Val() {
		go cm.addPageinfo(w)
	}
	// 将uid保存至redis
	go service.AddUID2Redis(date, w.Browsing.UID)
	w.WebFlow.Date = date
	w.WebFlow.URL = w.Pageinfo.URL
	w.WebFlow.Domain = w.Browsing.Domain
	w.Browsing.Date = date
	f := webFlowReq{
		webflow:  &w.WebFlow,
		browsing: &w.Browsing,
	}

	select {
	case cm.requests <- &f:
	case <-cm.quit:
		return
	}
}
func (cm *ConnManager) log(err error) {
	log.Println(err)
}

func (cm *ConnManager) addPageinfo(w *WebData) {
	// cm.pageinfos[w.Pageinfo.URL] = &w.Pageinfo
	go cm.handlePageinfo(w.Pageinfo)
}
func (cm *ConnManager) addPVRealtime(domain string, num int64) {
	cm.pvlock.Lock()
	cm.pvrealtime[domain] += num
	cm.pvlock.Unlock()
}
func (cm *ConnManager) addIPRealtime(domain, ip string, num int) {
	cm.iplock.Lock()
	if cm.iprealtime[domain] == nil {
		cm.iprealtime[domain] = make(map[string]interface{})
		cm.iprealtime[domain][ip] = 0
	}
	cm.iprealtime[domain][ip] = cm.iprealtime[domain][ip].(int) + num
	cm.iplock.Unlock()
}
func (cm *ConnManager) addUVRealtime(domain, uid string, num int) {
	cm.uvlock.Lock()
	if cm.uvrealtime[domain] == nil {
		cm.uvrealtime[domain] = make(map[string]interface{})
	}
	if cm.uvrealtime[domain][uid] == nil {
		cm.uvrealtime[domain][uid] = 0
	}
	cm.uvrealtime[domain][uid] = cm.uvrealtime[domain][uid].(int) + num
	cm.uvlock.Unlock()
}

// handleWebFlow 统计网络流量
func (cm *ConnManager) handleWebFlow(req *webFlowReq) {

	// 时段分析
	cm.addPVRealtime(req.browsing.Domain, 1)
	cm.addIPRealtime(req.browsing.Domain, req.browsing.IP, 1)
	cm.addUVRealtime(req.browsing.Domain, req.browsing.UID, 1)
	// pv
	req.webflow.PV++
	req.browsing.PV++
	// ip今天是否已经访问过了,
	if len(req.browsing.IP) > 0 && !isIPVisited(req.browsing.IP, req.webflow.URL, req.webflow.Date) {
		req.webflow.IP++
	}
	if len(req.browsing.UID) > 0 {
		// uv
		if !cm.isUVVisitedToday(req.browsing.UID, req.webflow.URL, req.webflow.Date) {
			req.webflow.UV++
			req.browsing.Depth++
		}
		// visits
		if !cm.isUVVisitedInHalfHour(req.browsing.UID, req.webflow.URL, req.webflow.Date) {
			req.webflow.Visits++
			req.browsing.Visits++
		}
		// NV 是否为新客户,默认为旧
		if cm.isNewVisitor(req.browsing.UID, req.webflow.Domain, req.webflow.Date) {
			req.browsing.NV = 1
		}
	}
	// Pageopend

	// 添加webflow到map
	// log.Printf("handleWebflow:%v\n", req.webflow)
	go cm.addWebflow(req.webflow)
	// 添加browsing 到 map
	go cm.addBrowsing(req.browsing)
}
func isIPVisited(ip, url, date string) bool {
	// 无需锁，因为同一个ip正常访问不存在并发问题
	key := service.GetRedisIPKey(date, ip)
	r := model.RedisCli.SIsMember(key, url)
	if !r.Val() {
		model.RedisCli.SAdd(key, url)
		// 明日凌晨过期
		model.RedisCli.ExpireAt(key, getTimeOfTomorrowZero(date))
	}
	return r.Val()
}
func (cm *ConnManager) isUVVisitedToday(uid, url, date string) bool {
	key := service.GetRedisVisitorKey(date, uid)
	r := model.RedisCli.SIsMember(key, url)
	if !r.Val() {
		model.RedisCli.SAdd(key, url)
		// 明日凌晨过期
		model.RedisCli.ExpireAt(key, getTimeOfTomorrowZero(date))
	}
	return r.Val()
}

// isUVVisited 查看用户半小时内是否已经访问过了
func (cm *ConnManager) isUVVisitedInHalfHour(uid, url, date string) bool {
	key := service.GetRedisVisitNumbersKey(date, uid)
	r := model.RedisCli.SIsMember(key, url)
	if !r.Val() {
		model.RedisCli.SAdd(key, url)
		err := model.RedisCli.Expire(key, 30*60*time.Second).Err()
		if err != nil {
			cm.log(err)
		}
	}
	return r.Val()
}

// isNewVisitor 访问历史数据库查询是否是新客户
func (cm *ConnManager) isNewVisitor(uid, domain, date string) bool {
	key := service.GetredisNewVisitorKey(date, domain)
	if !model.RedisCli.SIsMember(key, uid).Val() {
		b, err := service.IsNewVisitor(domain, uid)
		if err != nil {
			cm.log(err)
		}
		// 将新用户缓存至redis
		model.RedisCli.SAdd(key, uid)
		model.RedisCli.ExpireAt(key, getTimeOfTomorrowZero(date))
		return b
	}
	return true

}

// addWebflow 添加webflow
func (cm *ConnManager) addWebflow(data *model.WebFlow) {
	// 锁表
	cm.webflowsLock.Lock()
	// 判断是否存在,不存在就添加,否则
	wb := cm.webflows[data.URL+data.Date]
	if wb != nil {
		wb.PV += data.PV
		wb.IP += data.IP
		wb.UV += data.UV
		wb.Visits += data.Visits
		wb.Duration += data.Duration
		if len(data.Domain) > 0 {
			wb.Domain = data.Domain
		}
		cm.webflows[data.URL+data.Date] = wb
	} else {
		cm.webflows[data.URL+data.Date] = data
	}
	if len(cm.webflows[data.URL+data.Date].Domain) == 0 {
		cm.log(errors.New("domain 为空"))
	}
	// s, _ := util.ToJSONStr(wb)
	// fmt.Printf("key:%s,val:%v\n", (data.URL + data.Date), s)
	// 释放表
	cm.webflowsLock.Unlock()
	if len(cm.webflows) >= handlePerTime {
		cm.flushWebflowsToRedis()
	}
}

// addBrowsing 添加browsing
func (cm *ConnManager) addBrowsing(data *model.Browsing) {
	cm.browsingsLock.Lock()
	b := cm.browsings[data.UID+data.Date]
	// s1, _ := util.ToJSONStr(data)
	// fmt.Printf("data-key:%s,val:%v\n", (data.UID + util.FormatDate(data.CreateDate, util.YYYY_MM_DD)), s1)
	if b != nil {
		b.Depth += data.Depth
		b.PV += data.PV
		b.Visits += data.Visits
		b.Pageopend += data.Pageopend
	} else {
		cm.browsings[data.UID+data.Date] = data
	}
	cm.browsingsLock.Unlock()
	// s, _ := util.ToJSONStr(b)
	// fmt.Printf("key:%s,val:%v\n", (data.UID + util.FormatDate(data.CreateDate, util.YYYY_MM_DD)), s)
	if len(cm.browsings) >= handlePerTime {
		cm.flushBrowsingsToRedis()
	}
}

// flushWebflowToRedis 流量累加到redis
func (cm *ConnManager) flushWebflowsToRedis() {
	if len(cm.webflows) == 0 {
		return
	}
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
	key := service.GetRedisWebflowKey(webflow.Domain, webflow.Date, webflow.URL)
	// 要处理的事务
	txf := func(tx *redis.Tx) error {
		// 获取并更新值
		r := tx.HMGet(key, "PV", "IP", "UV", "Visits", "Duration")
		// 第二天凌晨过期
		tx.ExpireAt(key, getTimeOfTomorrowZero(webflow.Date))
		if r.Err() != nil && r.Err() != redis.Nil {
			return r.Err()
		}
		if r.Val()[0] != nil {
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
		// s1, _ := util.ToJSONStr(webflow)
		// log.Printf("webflow-after-val:%s\n", s1)

		// 存储到redis
		_, err := tx.Pipelined(func(pipe redis.Pipeliner) error {
			fields := map[string]interface{}{
				"PV":       webflow.PV,
				"IP":       webflow.IP,
				"UV":       webflow.UV,
				"Visits":   webflow.Visits,
				"Duration": webflow.Duration,
			}
			return pipe.HMSet(key, fields).Err()
		})
		return err
	}
	// 监测锁
	i := 0
	j := 0
	for {
		// 每隔1秒连续尝试100次
		i++
		if i > 100 { // 连续获取100次锁失败之后，隔1秒再获取
			time.Sleep(1 * time.Second)
			j++
			if j > 10 {
				log.Println("无法获取锁,10秒后将数据重新缓存")
				time.Sleep(10 * time.Second)
				cm.addWebflow(webflow)
				break
			}
			i = 0
		}
		err := model.RedisCli.Watch(txf, key)
		if err != redis.TxFailedErr {
			return
		}
	}
	cm.log(errors.New("达到最大重试次数"))
	return
}

// 用户浏览情况累加到redis
func (cm *ConnManager) flushBrowsingsToRedis() {
	if len(cm.browsings) == 0 {
		return
	}
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

	key := service.GetRedisBrowsingKey(browsing.Date, browsing.UID)
	// 要处理的事务
	txf := func(tx *redis.Tx) error {
		// 获取并更新值
		b, err := service.GetBrowsingFromRedis(key, browsing.Domain)
		if err != nil {
			return err
		}
		browsing.Depth += b.Depth
		browsing.PV += b.PV
		browsing.Visits += b.Visits
		browsing.Duration += b.Duration
		// 存储到redis
		_, err = tx.Pipelined(func(pipe redis.Pipeliner) error {
			// fields := map[string]interface{}{
			// 	"Depth":    browsing.Depth,
			// 	"PV":       browsing.PV,
			// 	"Visits":   browsing.Visits,
			// 	"Duration": browsing.Duration,
			// }
			// return pipe.HMSet(key, fields).Err()
			s, _ := util.ToJSONStr(browsing)
			err := pipe.HSet(key, browsing.Domain, s).Err()
			if err != nil {
				return err
			}

			return pipe.ExpireAt(key, getTimeOfTomorrowZero(browsing.Date)).Err()
		})
		return err
	}
	// 监测锁
	i, j := 0, 0
	for { // 每隔1秒连续尝试100次
		i++
		if i > 100 { // 连续获取100次锁失败之后，隔1秒再获取
			time.Sleep(1 * time.Second)
			j++
			if j > 10 {
				log.Println("无法获取锁,10秒后将数据重新缓存")
				time.Sleep(10 * time.Second)
				cm.addBrowsing(browsing)
				break
			}
			i = 0
		}
		err := model.RedisCli.Watch(txf, key)
		if err != redis.TxFailedErr {
			return
		}

	}
	cm.log(errors.New("达到最大重试次数"))
}
func (cm *ConnManager) flushRealtimeDataToRedis() {
	// pv
	go cm.flushPVRealtime2Redis()
	// ip
	go cm.flushIPRealtime2Redis()
	// uv
	go cm.flushUVRealtime2Redis()
}
func (cm *ConnManager) flushPVRealtime2Redis() {
	if len(cm.pvrealtime) == 0 {
		return
	}
	cm.pvlock.Lock()
	r := cm.pvrealtime
	cm.pvrealtime = make(map[string]int64)
	cm.pvlock.Unlock()
	for k, v := range r {
		go cm.persistPVRealtimeToRedis(k, v)
	}
}

// persistPVRealtimeToRedis 将实时pv缓存到redis
func (cm *ConnManager) persistPVRealtimeToRedis(domain string, val int64) {
	key := service.GetRedisTimePVKey(domain)
	txf := func(tx *redis.Tx) error {
		// 获取旧值
		n, err := tx.Get(key).Int64()
		if err != nil && err != redis.Nil {
			return err
		}
		n += val
		if n < 0 {
			n = 0
		}
		// 更新新值
		_, err = tx.Pipelined(func(pipe redis.Pipeliner) error {
			return pipe.Set(key, n, 0).Err()
		})
		return err
	}
	// 获取锁并更新值
	i, j := 0, 0
	for {
		i++
		if i > 10 {
			time.Sleep(1 * time.Second)
			j++
			if j > 10 {
				break
			}
			i = 0
		}
		err := model.RedisCli.Watch(txf, key)
		if err != redis.TxFailedErr {
			return
		}
	}
	cm.log(errors.New("increment reached maximum number of retries"))
}
func (cm *ConnManager) flushIPRealtime2Redis() {
	if len(cm.iprealtime) == 0 {
		return
	}
	cm.iplock.Lock()
	r := cm.iprealtime
	cm.iprealtime = make(map[string]map[string]interface{})
	cm.iplock.Unlock()
	for domain, vals := range r {
		go cm.persistIPRealtimeToRedis(domain, vals)
	}
}
func (cm *ConnManager) persistIPRealtimeToRedis(domain string, vals map[string]interface{}) {
	key := service.GetRedisTimeIPKey(domain)
	txf := func(tx *redis.Tx) error {
		// 更新值
		for ip := range vals {
			// 获取缓存的值
			r := tx.HMGet(key, ip)
			// 如果报错重新缓存数据
			if r.Err() != nil && r.Err() != redis.Nil {
				for k, v := range vals {
					cm.addIPRealtime(domain, k, v.(int))
				}
				return r.Err()
			}
			// 更新值
			if r.Val()[0] != nil {
				switch r.Val()[0].(type) {
				case int:
					vals[ip] = vals[ip].(int) + r.Val()[0].(int)
				case string:
					v, _ := strconv.Atoi(r.Val()[0].(string))
					vals[ip] = vals[ip].(int) + v
				}
			}
			// 删除为0的field
			if vals[ip].(int) <= 0 {
				delete(vals, ip)
				tx.HDel(key, ip)
			}
		}

		// 更新新值
		_, err := tx.Pipelined(func(pipe redis.Pipeliner) error {
			return pipe.HMSet(key, vals).Err()
		})
		return err
	}
	// 获取锁并更新值
	i, j := 0, 0
	for {
		i++
		if i > 10 {
			time.Sleep(1 * time.Second)
			j++
			if j > 10 {
				break
			}
			i = 0
		}
		err := model.RedisCli.Watch(txf, key)
		if err != redis.TxFailedErr {
			return
		}
	}
	cm.log(errors.New("increment reached maximum number of retries"))

}
func (cm *ConnManager) flushUVRealtime2Redis() {
	if len(cm.uvrealtime) == 0 {
		return
	}
	cm.uvlock.Lock()
	r := cm.uvrealtime
	cm.uvrealtime = make(map[string]map[string]interface{})
	cm.uvlock.Unlock()
	for domain, vals := range r {
		go cm.persistUVRealtimeToRedis(domain, vals)
	}
}
func (cm *ConnManager) persistUVRealtimeToRedis(domain string, vals map[string]interface{}) {
	key := service.GetRedisTimeUVKey(domain)
	txf := func(tx *redis.Tx) error {
		// 更新值
		for uid := range vals {
			// 获取缓存的值
			r := tx.HMGet(key, uid)
			// 如果报错重新缓存数据
			if r.Err() != nil && r.Err() != redis.Nil {
				for k, v := range vals {
					cm.addUVRealtime(domain, k, v.(int))
				}
				return r.Err()
			}
			// 更新值
			log.Println("uv:", r.Val())
			if r.Val()[0] != nil {

				switch r.Val()[0].(type) {
				case int:
					vals[uid] = vals[uid].(int) + r.Val()[0].(int)
				case string:
					v, _ := strconv.Atoi(r.Val()[0].(string))
					vals[uid] = vals[uid].(int) + v
				}
			}
			// 删除为0的field
			if vals[uid].(int) <= 0 {
				delete(vals, uid)
				tx.HDel(key, uid)
			}
		}

		// 更新新值
		_, err := tx.Pipelined(func(pipe redis.Pipeliner) error {
			return pipe.HMSet(key, vals).Err()
		})
		return err
	}
	// 获取锁并更新值
	i, j := 0, 0
	for {
		i++
		if i > 10 {
			time.Sleep(1 * time.Second)
			j++
			if j > 10 {
				break
			}
			i = 0
		}
		err := model.RedisCli.Watch(txf, key)
		if err != redis.TxFailedErr {
			return
		}
	}
	cm.log(errors.New("increment reached maximum number of retries"))
}

// handlePageinfo 保存page到redis和数据库,并发是否安全不影响,set只保留唯一值
func (cm *ConnManager) handlePageinfo(p model.Pageinfo) {
	// 保存pageinfo至redis
	defaultdate := util.GetDateAsDefaultStr()
	pipe := model.RedisCli.Pipeline()
	urlkey := service.GetRedisURLKey(defaultdate)
	pipe.SAdd(urlkey, p.URL)
	pageinfokey := service.GetRedisPageinfoKey(defaultdate, p.URL)
	s, _ := util.ToJSONStr(p)
	// fmt.Println(pageinfokey)
	pipe.Set(pageinfokey, s, 3600*24)
	_, err := pipe.Exec()
	if err != nil {
		cm.log(err)
	}
	// 保存到数据库
	if err = p.FirstOrCreate(); err != nil {
		cm.log(err)
	}
}
func (cm *ConnManager) handleDuration(d *Duration) {
	// 时段分析
	cm.addPVRealtime(d.Domain, -1)
	cm.addIPRealtime(d.Domain, d.IP, -1)
	cm.addUVRealtime(d.Domain, d.UID, -1)
	// 更新网页浏览时长
	cm.addWebflow(&model.WebFlow{
		URL:      d.URL,
		Date:     d.Date,
		Duration: d.Duration,
		Domain:   d.Domain,
	})
	// 更新用户今日浏览时长
	cm.addBrowsing(&model.Browsing{
		UID:      d.UID,
		Date:     d.Date,
		Duration: d.Duration,
		Domain:   d.Domain,
	})

}

// persistRealtimeWebflow 将实时网页流量持久化
func (cm *ConnManager) persistRealtimeWebflow() {
	// 查找注册域名

	result, err := service.GetRegistryDomains()
	if err != nil {
		cm.log(err)
	}
	if len(result) == 0 {
		return
	}
	for _, v := range result {
		go cm.persistRealtimeWebflowWithDomain(v.Domain)
	}
}

// persistRealtimeWebflowWithDomain 持久化指定域名实时网页流量
func (cm *ConnManager) persistRealtimeWebflowWithDomain(domain string) {
	// 从redis获取实时pv,ip,uv\
	p := model.RedisCli.Get(service.GetRedisTimePVKey(domain))
	if p.Err() != nil {
		cm.log(p.Err())
	}
	pv, _ := strconv.Atoi(p.Val())

	i := model.RedisCli.HLen(service.GetRedisTimeIPKey(domain))
	if i.Err() != nil {
		cm.log(i.Err())
	}
	ip := i.Val()

	u := model.RedisCli.HLen(service.GetRedisTimeUVKey(domain))
	if u.Err() != nil {
		cm.log(u.Err())
	}
	uv := u.Val()
	// 持久化
	data := model.RealtimeWebflow{
		PV:     pv,
		IP:     ip,
		UV:     uv,
		Domain: domain,
		Date:   time.Now().Format("2006-01-02 15:04"),
	}
	err := service.SaveRealtimeWebflow(&data)
	if err != nil {
		cm.log(err)
	}
}

// getTimeOfTomorrowZero 获取明天0点的timestamp
func getTimeOfTomorrowZero(datestr string) time.Time {
	d, _ := util.ParseDate(datestr, util.YYYY_MM_DD)
	d = d.Add(time.Hour * 24)
	return d
}
