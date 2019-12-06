package model

// Pageinfo 页面信息
type Pageinfo struct {
	Model
	Dm    string `json:"dm"`    // 域名
	URL   string `json:"url"`   // 网址
	Title string `json:"title"` // 标题
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
