// INPUT: Server 组合根注册的 Chi endpoint handler 与浏览器编码后的动态路径段。
// OUTPUT: 路由匹配后、进入领域 Handler 前已解码一次的路径参数。
// POS: HTTP route composition 边界；确保所有当前及后续直接注册的 API 路由共享同一解码契约。
package server

import (
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"

	"github.com/go-chi/chi/v5"
)

// provider model_id 在 service 中仍需兼容历史上已持久化的转义 ID，
// 因此它继续由该唯一旧边界解码；其余路径参数统一由 Handler 边界处理。
const serviceDecodedModelIDPathParam = "model_id"

type pathParamRouter struct {
	chi.Router
}

var _ chi.Router = (*pathParamRouter)(nil)

func newPathParamRouter() chi.Router {
	return &pathParamRouter{Router: chi.NewRouter()}
}

func (r *pathParamRouter) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.RawPath == "" {
		// net/url 会在原始路径恰好等于 Path 的标准转义形式时省略 RawPath。
		// 例如浏览器发送 literal%252Fvalue，URL.Path 是 literal%2Fvalue；
		// 重新固定 EscapedPath 后，Chi 才不会把字面量百分号误当成下一层转义。
		request.URL.RawPath = request.URL.EscapedPath()
	}
	r.Router.ServeHTTP(writer, request)
}

func (r *pathParamRouter) decoded(handler http.Handler) http.Handler {
	return handlershared.DecodePathParams(handler, serviceDecodedModelIDPathParam)
}

func (r *pathParamRouter) handler(handler http.HandlerFunc) http.HandlerFunc {
	return r.decoded(handler).ServeHTTP
}

func (r *pathParamRouter) With(middlewares ...func(http.Handler) http.Handler) chi.Router {
	return &pathParamRouter{Router: r.Router.With(middlewares...)}
}

func (r *pathParamRouter) Group(fn func(chi.Router)) chi.Router {
	var wrapped chi.Router
	r.Router.Group(func(router chi.Router) {
		wrapped = &pathParamRouter{Router: router}
		fn(wrapped)
	})
	return wrapped
}

func (r *pathParamRouter) Route(pattern string, fn func(chi.Router)) chi.Router {
	var wrapped chi.Router
	r.Router.Route(pattern, func(router chi.Router) {
		wrapped = &pathParamRouter{Router: router}
		fn(wrapped)
	})
	return wrapped
}

func (r *pathParamRouter) Handle(pattern string, handler http.Handler) {
	r.Router.Handle(pattern, r.decoded(handler))
}

func (r *pathParamRouter) HandleFunc(pattern string, handler http.HandlerFunc) {
	r.Router.HandleFunc(pattern, r.handler(handler))
}

func (r *pathParamRouter) Method(method string, pattern string, handler http.Handler) {
	r.Router.Method(method, pattern, r.decoded(handler))
}

func (r *pathParamRouter) MethodFunc(method string, pattern string, handler http.HandlerFunc) {
	r.Router.MethodFunc(method, pattern, r.handler(handler))
}

func (r *pathParamRouter) Connect(pattern string, handler http.HandlerFunc) {
	r.Router.Connect(pattern, r.handler(handler))
}

func (r *pathParamRouter) Delete(pattern string, handler http.HandlerFunc) {
	r.Router.Delete(pattern, r.handler(handler))
}

func (r *pathParamRouter) Get(pattern string, handler http.HandlerFunc) {
	r.Router.Get(pattern, r.handler(handler))
}

func (r *pathParamRouter) Head(pattern string, handler http.HandlerFunc) {
	r.Router.Head(pattern, r.handler(handler))
}

func (r *pathParamRouter) Options(pattern string, handler http.HandlerFunc) {
	r.Router.Options(pattern, r.handler(handler))
}

func (r *pathParamRouter) Patch(pattern string, handler http.HandlerFunc) {
	r.Router.Patch(pattern, r.handler(handler))
}

func (r *pathParamRouter) Post(pattern string, handler http.HandlerFunc) {
	r.Router.Post(pattern, r.handler(handler))
}

func (r *pathParamRouter) Put(pattern string, handler http.HandlerFunc) {
	r.Router.Put(pattern, r.handler(handler))
}

func (r *pathParamRouter) Trace(pattern string, handler http.HandlerFunc) {
	r.Router.Trace(pattern, r.handler(handler))
}
