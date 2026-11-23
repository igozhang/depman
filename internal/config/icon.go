package config

import "sync"

// DefaultIconSVG：红黄蓝三色底 + 高对比 CICD（<2KB）。
const DefaultIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64">
<rect width="64" height="64" rx="12" fill="#111"/>
<rect x="5" y="8" width="17" height="48" rx="3" fill="#e52521"/>
<rect x="23.5" y="8" width="17" height="48" rx="3" fill="#f5c518"/>
<rect x="42" y="8" width="17" height="48" rx="3" fill="#049cd8"/>
<rect x="8" y="24" width="48" height="18" rx="4" fill="#111" fill-opacity=".72"/>
<text x="32" y="37.5" text-anchor="middle" font-family="Segoe UI,Helvetica,Arial,sans-serif" font-size="15" font-weight="800" letter-spacing="1.2" fill="#fff">CICD</text>
</svg>`

var (
	defaultIconOnce sync.Once
	defaultIconSVG  []byte
)

func DefaultIconBytes() []byte {
	defaultIconOnce.Do(func() {
		defaultIconSVG = []byte(DefaultIconSVG)
	})
	return defaultIconSVG
}
