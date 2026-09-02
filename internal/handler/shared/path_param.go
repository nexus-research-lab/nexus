// INPUT: Chi 已完成路由匹配的 HTTP 请求与单个路径参数名。
// OUTPUT: 与前端 encodeURIComponent 对称、且只执行一次的路径段解码结果。
// POS: Handler 共享传输边界；除显式历史兼容参数外，service 只接收真实资源标识。
package shared

import (
	"context"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

type decodedPathParamsContextKey struct{}

// DecodePathParams 在 Chi 完成路由匹配后统一解码动态路径段。
// preservedNames 仅用于仍在 service 层承担一次历史兼容解码的旧参数。
func DecodePathParams(next http.Handler, preservedNames ...string) http.Handler {
	preserved := make(map[string]struct{}, len(preservedNames))
	for _, name := range preservedNames {
		preserved[name] = struct{}{}
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		routeContext := chi.RouteContext(request.Context())
		settledNames := make(map[string]struct{}, len(routeContext.URLParams.Keys))
		for name := range preserved {
			settledNames[name] = struct{}{}
		}
		for index, name := range routeContext.URLParams.Keys {
			if _, keepEncoded := preserved[name]; keepEncoded || index >= len(routeContext.URLParams.Values) {
				continue
			}
			value := routeContext.URLParams.Values[index]
			decoded, err := url.PathUnescape(value)
			if err != nil {
				continue
			}
			routeContext.URLParams.Values[index] = decoded
			request.SetPathValue(name, decoded)
			settledNames[name] = struct{}{}
		}
		ctx := context.WithValue(request.Context(), decodedPathParamsContextKey{}, settledNames)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// PathParam 返回共享边界已经结算的 Chi 路径参数；独立 Handler 调用时解码一次。
//
// Chi 使用 URL.RawPath 完成路由，以便被编码的斜杠仍属于同一个路径段；
// 因此 URLParam 保留了百分号转义。浏览器端对动态路径段使用
// encodeURIComponent，这里是对应的唯一解码边界。显式保留给历史 service
// 兼容层的参数直接返回当前值，避免未来改用本辅助函数后发生二次解码。
func PathParam(request *http.Request, name string) string {
	value := chi.URLParam(request, name)
	if settledNames, ok := request.Context().Value(decodedPathParamsContextKey{}).(map[string]struct{}); ok {
		if _, settled := settledNames[name]; settled {
			return value
		}
	}
	decoded, err := url.PathUnescape(value)
	if err != nil {
		// net/http 会拒绝非法 URL 转义；保留原值也让直接构造请求的调用方
		// 继续交由领域校验拒绝，而不是把身份改写成另一个资源。
		return value
	}
	return decoded
}
