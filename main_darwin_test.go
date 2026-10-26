package main

import (
	"bytes"
	"image/png"
	"testing"
)

// TestMacApplicationIcon 验证交给 Wails 的运行时图标保留 macOS 透明留边
func TestMacApplicationIcon(t *testing.T) {
	icon, err := png.Decode(bytes.NewReader(appIcon))
	if err != nil {
		t.Fatal(err)
	}
	bounds := icon.Bounds()
	for _, point := range [][2]int{{0, 0}, {bounds.Dx() / 2, 0}, {0, bounds.Dy() / 2}, {bounds.Dx() - 1, bounds.Dy() - 1}} {
		_, _, _, alpha := icon.At(point[0], point[1]).RGBA()
		if alpha != 0 {
			t.Fatalf("Wails 运行时图标在 (%d, %d) 不透明，启动后会覆盖应用包的圆角图标", point[0], point[1])
		}
	}
	_, _, _, alpha := icon.At(bounds.Dx()/2, bounds.Dy()/2).RGBA()
	if alpha == 0 {
		t.Fatal("运行时图标中心不应透明")
	}
}
