package gin

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangxw06/go-butler/internal/contexts"
)

const (
	NoneLogContent LogContent = iota
	MetadataLogContent
	FullLogContent
)

type (
	LogContent = int
)

var (
	path2LogContent = make(map[string]LogContent)
)

func HandleAndLog(group *gin.RouterGroup, httpMethod, relativePath string, logContent LogContent, handlers ...gin.HandlerFunc) gin.IRoutes {
	basePath := group.BasePath()
	path := basePath + relativePath
	path2LogContent[path] = logContent
	return group.Handle(httpMethod, relativePath, handlers...)
}

func HandleAnyAndLog(group *gin.RouterGroup, relativePath string, logContent LogContent, handlers ...gin.HandlerFunc) gin.IRoutes {
	basePath := group.BasePath()
	path := basePath + relativePath
	path2LogContent[path] = logContent
	return group.Any(relativePath, handlers...)
}

func LoggerMiddleWare(c *gin.Context) {
	//var bodyBytes []byte
	//if c.Request.Body != nil {
	//	bodyBytes, _ = ioutil.ReadAll(c.Request.Body)
	//}
	//c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	c.Next()

	logContent, ok := path2LogContent[c.FullPath()]
	if !ok || logContent == NoneLogContent {
		return
	}
	switch logContent {
	case MetadataLogContent:
		contexts.Logger(c).With(
			"clientIP", c.ClientIP(),
			"url", c.Request.URL,
			"status", c.Writer.Status()).Info("gin access")
	case FullLogContent:
		contexts.Logger(c).With(
			"clientIP", c.ClientIP(),
			"urlQuery", c.Request.URL.Query(),
			//"body", string(bodyBytes),
			"postForm", c.Request.PostForm,
			"status", c.Writer.Status(),
			"response", string(assembleBody(c))).Info("gin access")
	default:

	}
}
