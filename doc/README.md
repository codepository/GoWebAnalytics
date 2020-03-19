## 统计逻辑

本地内存先缓存->每隔10秒或者本地缓存一定数量时保存到redis->每天0点将redis信息写入数据库

本地内存先缓存： 减少访问redis的频次

缓存到redis: 提升读写速度，且可以实现分布式并发更新



// WebData 页面信息
type WebData struct {
	Pageinfo model.Pageinfo `json:"p"`
	WebFlow  model.WebFlow  `json:"w"`
	Browsing model.Browsing `json:"b"`
	Type     string         `json:"t"`
}

## 页面信息

```
// Pageinfo 页面信息
type Pageinfo struct {
  Dm  string `json:"dm"`  // 域名
	URL string `json:"url"` // 网址
	// Keywords 关键词
	Keywords      string `json:"keywords"`
	Description   string `json:"description"`
	Filetype      int8   `json:"filetype"`
	Publishedtype int8   `json:"publishedtype"`
	Pagetype      int8   `json:"pagetype"`
	Catalogs      string `json:"catalogs"`
	Contentid     string `json:"contentid"`
	Publishdate   string `json:"publishdate"`
	Author        string `json:"author"`
	Source        string `json:"source"`
}
```


## 流量统计

```
// WebFlow 网页流量
type WebFlow struct {
	Model
	URL    string `json:"url"`    // 网址
	PV     int    `json:"pv"`     // 页面浏览量
	IP     int    `json:"ip"`     // 访问ip数
	UV     int    `json:"uv"`     // 独立访问者数
	Visits int    `json:"visits"` // 访问次数(半个小时内多次算一次)
	BR     int    `json:"br"`     // Bounce Rate 跳出率,只访问一次就跳出
}
```

## 访客习惯

```
// Browsing 用户访问习惯
type Browsing struct {
	UID        string    `json:"uid"`        // 用户id
	Depth      int       `json:"depth"`      // 访问页面数
	PV         int       `json:"pv"`         // 页面浏览量
	Visits     int       `json:"visits"`     // 访问次数(半个小时内多次算一次)
	Duration   uint64    `json:"duration"`   // 浏览时长
	Pageopend  int       `json:"pageopend"`  // 同时打开页面数
	Region     string    `json:"region"`     // 区域
	OS         string    `json:"os"`         // 操作系统
	Browser    string    `json:"browser"`    // 浏览器
	DeviceType int       `json:"deviceType"` // 终端类型 0为电脑、1为手机
	SR         string    `json:"sr"`         // 屏幕分辨率
	Start      time.Time `json:"start"`      // 开始时间
	NV         int       `json:"nv"`         // new visitor 0为用户回访,1为今天新访客
}
```

## 网页流量统计流程图

<img src="./img/网页流量分析-流量分析流程图.png"/>


## redis 缓存


#### 网页页面信息
https://www.cnblogs.com/xujishou/p/6423453.html
<!-- hashmap -->
<!-- 24小时后过期 -->
tongji_pageinfo_<yyyy-mm-dd>_<url>:Pageinfo // 用于纪录页面信息


#### 网页流量统计
<!-- set -->
tongji_url_<yyyy-mm-dd>：url // 通过 sort tongj_url_<yyyy-mm-dd> by tongji_webflow_<domain>_<yyyy-mm-dd>-*->pv desc根据流量降序排序
<!-- hashmap -->
<!-- 第二天凌晨过期 -->
tongji_webflow_<domain>_<yyyy-mm-dd>-<url>: WebFlow   // 统计指定url指定日期的流量

<!-- set -->
<!-- 第二天凌晨过期 -->
tongji_ip_<yyyy-mm-dd>_<ip>: url // 统计指定ip访问过的页面


#### 访客习惯
<!-- set -->
<!-- 半小时内自动过期 -->
tongji_visitnumbers_url_<yyyy-mm-dd>_<visitor> 统计用户访问次数的key,半小时后自动过期
<!-- set -->
tongji_uid_<yyyy-mm-dd>: uid // 用于纪录今日访问的用户id
<!-- set-->
<!-- 第二天凌晨过期 -->
tongji_visitor_url_<yyyy-mm-dd>_<visitor>: url   // 统计独立用户查看过的页面，用于统计浏览深度
<!-- hashmap -->
<!-- 第二天凌晨过期 -->
tongji_browsing_<yyyy-mm-dd>_<visitor>: domain:Browsing //用于统计独立用户的访问习惯
<!-- set -->
<!-- 第二天凌晨过期 -->
tongji_newvisitor_<yyyy-mm-dd>_<domain>: uid // 用于统计今日新用户

#### 跳出率统计

<!-- set 隔日过期--> 用于统计一日内
tongji_domain_bounce_<yyyy-mm-dd>: uid

#### 时段分析:pv、uv、ip
<!-- string 统计实时打开页面数，打开一个页面加1，关闭一个页面减1 -->
tongji_time_<domain>_pv: <打开页面数>
<!-- hashmap -->
tongji_time_<domain>_ip: <ip>:<打开页面数>
tongji_time_<domain>_uv: <uid>:<打开页面数>



## 持久化数据到Mysql

### 持久化

#### 保存链接信息


#### 存储url对应的webflow

1、通过 SPopN 从redis获取key为tongji_url_<yyyy-mm-dd> 的集合中保存的url

2、已知url，从redis获取key为tongji_webflow_<domain>_<yyyy-mm-dd>-<url>中保存的webflow

3、保存到数据库

### 存储用户浏览习惯

1、通过 SPopN 从redis获取key为tongji_uid_<yyyy-mm-dd>的集合中保存的uid

2、根据uid,从redis获取key为tongji_browsing_<yyyy-mm-dd>_<visitor>的hashmap中保存的browsing

3、保存到数据库


### 删除redis上的key 和 过时时间设定




