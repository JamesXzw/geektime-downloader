package config

import "fmt"

type Config struct {
	GCID      string   `json:"gcid"`
	GCESS     string   `json:"gcess"`
	CourseIDs []string `json:"course_ids"`
}

const (
	GeektimeDownloaderFolder = "geektime"
	DefaultDownloadPath      = "/Users/bytedance/geektime"
)

// GetConfig 从配置文件读取配置
// todo xu 下载完之后需要再确认一遍所有课程都下载成功，避免有遗漏
func GetConfig() (*Config, error) {
	fmt.Println("Debug: Loading config...") // 添加调试信息
	cfg := &Config{
		GCID:  "d07e360-d12ac62-9995ae6-4e6bd3f",
		GCESS: "BgEIBsAYAAAAAAALAgYABQQAAAAAAgQ0doRnBwQIR2xHCQEBDAEBCAEDCgQAAAAAAwQ0doRnDQEBBAQAjScABgR7U8vZ",
		CourseIDs: []string{
			"100043001", "100081501", "100034501", "100026801", "100032301",
			"100029201", "100003101", "100020301", "100029001", "100014401",
			"100029501", "100026901", "100021201", "100012101", "100552001",
			"100111101", "100034101", "100104301", "100040201", "100035801",
			"100636605", "100023701", "100099801", "100105701", "100037301",
			"100014301", "100007101", "100064501", "100025201", "100094901",
			"100019601", "100626901", "100124001", "100090601", "100056701",
			"100083301", "100078501", "100550701", "100555001", "100617601",
			"100625601", "100633001", "100038501", "100046301", "100036501",
			"100007001", "100015201", "100046201", "100069901", "100114001",
			"100309001", "100020901", "100035801", "100041701", "100046801",
			"100053901", "100020201", "100003901", "100804101", "100839101",
			"100117801", "100026001", "100061801", "100006601", "100541001",
			"100613101", "100024701", "100082101", "100002201", "100039001",
			"100085201", "100755401", "100085101", "100020801", "100062401",
			"100093501", "100013101", "100022301", "100079601", "100017301",
			"100068401", "100052601", "100311801", "100102601", "100084301",
			"100079101", "100033601", "100009601", "100120501", "100093001",
		},
	}
	fmt.Printf("Debug: Config loaded - GCID: %s\n", cfg.GCID) // 添加调试信息
	return cfg, nil
}
