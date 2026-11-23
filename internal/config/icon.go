package config

import "sync"

// DefaultIconSVG：红黄蓝三色 CICD 标（<2KB），无自有 icon 时作 favicon / 页眉。
const DefaultIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64">
<rect width="64" height="64" rx="12" fill="#1a1a1a"/>
<rect x="6" y="10" width="16" height="44" rx="3" fill="#e52521"/>
<rect x="24" y="10" width="16" height="44" rx="3" fill="#f5c518"/>
<rect x="42" y="10" width="16" height="44" rx="3" fill="#049cd8"/>
<text x="32" y="38" text-anchor="middle" font-family="Segoe UI,Helvetica,Arial,sans-serif" font-size="13" font-weight="700" letter-spacing="0.6" fill="#ffffff">CICD</text>
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
