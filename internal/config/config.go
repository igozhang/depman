package config

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr    string
	DBPath  string
	DataDir string
	EtcDir  string
	Title   string
}

type UISettings struct {
	Title      string
	Tagline    string
	BannerPath string
	IconPath   string
}

const defaultTitle = "depman"
const FixedTagline = "简单发布"

const defaultConfBody = `addr=:8080
data_dir=./data
title=depman

# =============================================================================
# 说明（以下均为注释，修改请改上方生效区）
# =============================================================================
# 格式：key=value，# 开头为注释。环境变量 DEPMAN_* 优先级更高。
#
# | 配置项   | 环境变量        | 默认                 | 说明 |
# |----------|-----------------|----------------------|------|
# | addr     | DEPMAN_ADDR     | :8080                | HTTP 监听 |
# | data_dir | DEPMAN_DATA_DIR | ./data               | 数据目录 |
# | db_path  | DEPMAN_DB_PATH  | <data_dir>/depman.db | SQLite 路径，可省略 |
# | title    | （仅 conf）     | depman               | 页眉品牌名；兼浏览器标题 |
# | icon     | （仅 conf）     | （自动探测）         | etc 下站点 icon 文件名 |
# | banner   | （仅 conf）     | （自动探测）         | etc 下 banner 图片 |
#
# 页眉 slogan 固定为「简单发布」；双击可复制超管 igokey。
#
# 鉴权：
#   默认只读。GET /api/v1/auth/igokey 取得 8 位字母超管密钥（约 1 小时）。
#   在「登录」页验证后可写；超管可签发项目管理 key（绑定项目/环境）。
#   密钥落在 <data_dir>/igokey-active.json。
#
# 站点 icon：将 icon.svg / icon.png 放到 etc；无文件则使用内置 CICD SVG。
#

`

func Load() Config {
	etcDir := getenv("DEPMAN_ETC_DIR", "./etc")
	if err := EnsureDefaultConf(etcDir); err != nil {
		log.Printf("ensure etc conf: %v", err)
	}
	fileVals := readConfFile(filepath.Join(etcDir, "depman.conf"))

	dataDir := firstNonEmpty(os.Getenv("DEPMAN_DATA_DIR"), fileVals["data_dir"], "./data")
	dbPath := firstNonEmpty(os.Getenv("DEPMAN_DB_PATH"), fileVals["db_path"], filepath.Join(dataDir, "depman.db"))
	addr := firstNonEmpty(os.Getenv("DEPMAN_ADDR"), fileVals["addr"], ":8080")
	title := firstNonEmpty(fileVals["title"], defaultTitle)

	return Config{
		Addr:    addr,
		DBPath:  dbPath,
		DataDir: dataDir,
		EtcDir:  etcDir,
		Title:   title,
	}
}

func EnsureDefaultConf(etcDir string) error {
	if err := os.MkdirAll(etcDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(etcDir, "depman.conf")
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return nil
	}
	return os.WriteFile(path, []byte(defaultConfBody), 0o644)
}

func LoadUI(etcDir string) UISettings {
	vals := readConfFile(filepath.Join(etcDir, "depman.conf"))
	title := firstNonEmpty(vals["title"], defaultTitle)
	icon := resolveMedia(etcDir, vals["icon"], []string{"icon.svg", "icon.png", "icon.jpg", "icon.jpeg"})
	banner := resolveMedia(etcDir, vals["banner"], []string{"banner.jpg", "banner.jpeg", "banner.png", "banner.webp", "banner.gif"})
	return UISettings{
		Title:      title,
		Tagline:    FixedTagline,
		IconPath:   icon,
		BannerPath: banner,
	}
}

func resolveMedia(etcDir, named string, candidates []string) string {
	if named != "" {
		p := filepath.Join(etcDir, named)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	for _, name := range candidates {
		p := filepath.Join(etcDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func readConfFile(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		out[k] = v
	}
	return out
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
