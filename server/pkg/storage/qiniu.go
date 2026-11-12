// Package storage 对象存储抽象(当前实现:七牛云 Kodo)。
// 由后端中转上传:管理端传图 → API → 七牛 → 返回外链 URL,数据库只存 URL。
package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"

	"fawn-mall/pkg/config"
)

// Uploader 图片上传后端。UploadImage 将内容上传到 key 并返回可访问的完整 URL。
type Uploader interface {
	UploadImage(ctx context.Context, key string, r io.Reader, size int64) (string, error)
}

type qiniuUploader struct {
	mac      *auth.Credentials
	bucket   string
	domain   string
	uploader *storage.FormUploader
}

// NewQiniu 按配置构建七牛上传器(cfg 已通过 config.Validate 校验)。
func NewQiniu(cfg config.QiniuConf) (Uploader, error) {
	zone, err := zoneByRegion(cfg.Region)
	if err != nil {
		return nil, err
	}
	return &qiniuUploader{
		mac:      auth.New(cfg.AccessKey, cfg.SecretKey),
		bucket:   cfg.Bucket,
		domain:   strings.TrimSuffix(cfg.Domain, "/"),
		uploader: storage.NewFormUploader(&storage.Config{UseHTTPS: cfg.UseHTTPS, Region: zone}),
	}, nil
}

// UploadImage 表单上传。scope 限定到精确 key(bucket:key),同名重传=覆盖,
// 凭证本身无法写其他 key,泄露影响面最小。
func (u *qiniuUploader) UploadImage(ctx context.Context, key string, r io.Reader, size int64) (string, error) {
	putPolicy := storage.PutPolicy{
		Scope:   u.bucket + ":" + key,
		Expires: 600, // 凭证有效期 10 分钟(SDK 内部换算为绝对时间戳)
	}
	var ret storage.PutRet
	err := u.uploader.Put(ctx, &ret, putPolicy.UploadToken(u.mac), key, r, size, &storage.PutExtra{})
	if err != nil {
		return "", fmt.Errorf("七牛上传 %s: %w", key, err)
	}
	return u.domain + "/" + key, nil
}

// zoneByRegion 区域代码 → SDK Region。
func zoneByRegion(region string) (*storage.Region, error) {
	switch region {
	case "z0":
		return &storage.ZoneHuadong, nil // 华东(杭州)
	case "z1":
		return &storage.ZoneHuabei, nil // 华北(北京)
	case "z2":
		return &storage.ZoneHuanan, nil // 华南(广州)
	case "cn-east-2":
		return &storage.ZoneHuadongZheJiang2, nil // 华东-浙江2
	case "na0":
		return &storage.ZoneBeimei, nil // 北美
	case "as0":
		return &storage.ZoneXinjiapo, nil // 东南亚(新加坡)
	}
	return nil, fmt.Errorf("不支持的七牛区域代码 %q(可选 z0/z1/z2/cn-east-2/na0/as0)", region)
}

// ImageKey 生成对象存储 key:products/{日期}/{16位随机hex}.{ext}
// 随机名避免撞车与中文/特殊字符转义问题;日期分目录便于按天清理。
func ImageKey(ext string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("products/%s/%s.%s", time.Now().Format("20060102"), hex.EncodeToString(b), ext)
}
