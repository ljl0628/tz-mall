package handler

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"fawn-mall/pkg/apperr"
	"fawn-mall/pkg/resp"
	"fawn-mall/pkg/storage"
)

// maxImageSize 单图上限 5MB(管理端商品图,服务端中转足够)。
const maxImageSize = 5 << 20

// imageTypes 允许的图片类型:魔数嗅探结果 → 对象扩展名。
var imageTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// UploadImage POST /admin/upload/image(multipart 字段名 file)
// 服务端中转上传七牛:大小/魔数校验 → 随机 key → 返回外链 URL。
// 前端拿到 URL 后照旧写入商品表单的 image 字段,商品链路零改动。
func (h *AdminHandler) UploadImage(c *gin.Context) {
	if h.uploader == nil {
		resp.Fail(c, resp.NewBiz(resp.CodeImageUpload, "图片存储未配置:服务端缺少 qiniu.* 配置"))
		return
	}
	// 上限 = 图片 5MB + multipart 编码开销,超限直接断开读取
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageSize+64<<10)
	f, _, err := c.Request.FormFile("file")
	if err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "读取文件失败:multipart 字段名须为 file,单图上限 5MB"))
		return
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "读取文件内容失败"))
		return
	}
	if len(buf) == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "文件内容为空"))
		return
	}
	if len(buf) > maxImageSize {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "图片超过 5MB 上限"))
		return
	}
	// 按魔数判定真实类型,不信 Content-Type/扩展名
	head := buf
	if len(head) > 512 {
		head = head[:512]
	}
	ct := http.DetectContentType(head)
	ext, ok := imageTypes[ct]
	if !ok {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "仅支持 jpg/png/webp,检测到类型: "+ct))
		return
	}

	key := storage.ImageKey(ext)
	url, err := h.uploader.UploadImage(c.Request.Context(), key, bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		resp.Fail(c, apperr.Wrap("admin 上传商品图", err))
		return
	}
	resp.OK(c, gin.H{"url": url, "key": key, "content_type": ct, "size": len(buf)})
}
