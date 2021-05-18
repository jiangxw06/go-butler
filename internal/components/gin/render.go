package gin

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"html/template"
	"net/http"
)

func SetBody(ctx *gin.Context, obj interface{}) {
	b, _ := json.Marshal(obj)
	callback := ctx.Query("callback")
	if callback != "" {
		b = []byte(template.JSEscapeString(callback) + "(" + string(b) + ")")
	}
	ctx.Set("json", b)
}

func renderResp(ctx *gin.Context, body []byte) {
	ctx.Render(ctx.Writer.Status(),
		render.Data{
			ContentType: "application/json",
			Data:        body,
		})
}

func assembleBody(ctx *gin.Context) (resp []byte) {
	if ctx.Writer.Status() == http.StatusOK {
		data, ok := ctx.Get("json")
		if ok {
			resp = data.([]byte)
		}
	} else {
		detail := ""
		switch ctx.Writer.Status() {
		case http.StatusBadRequest:
			detail = "bad request, request params invalid"
		case http.StatusInternalServerError:
			detail = ctx.GetString("errorDetail")
			if len(detail) == 0 {
				detail = "server internal exception"
			}
		default:
			detail = fmt.Sprintf("http status: %v", ctx.Writer.Status())
		}
		resp = []byte(detail)
	}

	//debug模式
	//todo 将debug日志渲染为json格式和resp合并起来
	mode := ctx.Query("debug")
	if mode == "on" {
		logStr := fmt.Sprintf("\n%v", contexts.GetDebugModeLogs(ctx))
		resp = append(resp, []byte(logStr)...)
	}

	return
}
