package storage

import (
	"regexp"
	"testing"
)

// TestImageKey key 格式:products/{yyyymmdd}/{16位hex}.{ext},两次生成不同名。
func TestImageKey(t *testing.T) {
	re := regexp.MustCompile(`^products/\d{8}/[0-9a-f]{16}\.(jpg|png|webp)$`)
	for _, ext := range []string{"jpg", "png", "webp"} {
		k := ImageKey(ext)
		if !re.MatchString(k) {
			t.Fatalf("key 格式不符: %q", k)
		}
	}
	if ImageKey("jpg") == ImageKey("jpg") {
		t.Fatal("两次生成的 key 不应相同(随机名失效)")
	}
}

// TestZoneByRegion 全部合法区域码可解析,非法码报错。
func TestZoneByRegion(t *testing.T) {
	for _, r := range []string{"z0", "z1", "z2", "cn-east-2", "na0", "as0"} {
		if _, err := zoneByRegion(r); err != nil {
			t.Fatalf("合法区域码 %q 解析失败: %v", r, err)
		}
	}
	if _, err := zoneByRegion("z9"); err == nil {
		t.Fatal("非法区域码 z9 应报错")
	}
}
