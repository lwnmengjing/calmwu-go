/*
 * @Author: calmwu
 * @Date: 2017-11-14 11:11:02
 * @Last Modified by: calmwu
 * @Last Modified time: 2017-12-08 17:28:10
 */

package utils

import (
	"github.com/wubo0067/calmwu-go/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)


//WebInterfaceInfo web接口方法的描述
type WebInterfaceInfo struct {
	HTTPMethodType string
	HandlerFunc    gin.HandlerFunc
}

//WebItfMap 接口集合
type WebItfMap map[string]*WebInterfaceInfo

// RegisterWebItfsToGin 注册到gin
func RegisterWebItfsToGin(router *gin.Engine, webItfMap WebItfMap) {
	var ginHandlerFunc func(string, ...gin.HandlerFunc)

	for webItfPath, webItfInfo := range webItfMap {
		switch webItfInfo.HTTPMethodType {
		case http.MethodGet:
			ginHandlerFunc = router.GET
		case http.MethodPost:
			ginHandlerFunc = router.POST
		case http.MethodPut:
			ginHandlerFunc = router.PUT
		case http.MethodDelete:
			ginHandlerFunc = router.DELETE
		default:
			utils.ZLog.Errorf("ItfPath:%s MethodType:%s not support!", webItfPath, wetItfInfo.HTTPMethodType)
			continue
		}

		ginHandlerFunc(webItfPath, webItfInfo.HandlerFunc)
		utils.ZLog.Infof("Register ItfPath:%s MethodType:%s to GinRouter", webItfPath, wetItfInfo.HTTPMethodType)
	}
}

// RegisterWebItf 接口注册
func RegisterWebItf(webItfPath string, httpMethodType string, handlerFunc gin.HandlerFunc, webItfMap WebItfMap) {
	if _, ok := webItfMap[webItfPath]; !ok {
		webItfInfo := &WebInterfaceInfo{
			HTTPMethodType: httpMethodType,
			HandlerFunc:    handlerFunc,
		}
		webItfMap[webItfPath] = webItfInfo
	}
}
