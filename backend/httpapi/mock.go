package httpapi

import (
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
)

var mockErrors = [...]struct {
	status  int
	message string
}{
	{http.StatusBadRequest, "模拟故障：请求参数格式错误，无法解析诊断查询条件"},
	{http.StatusUnauthorized, "模拟故障：访问凭据已过期，身份认证失败"},
	{http.StatusForbidden, "模拟故障：当前账号无权访问目标资源"},
	{http.StatusNotFound, "模拟故障：目标实例不存在或已被移除"},
	{http.StatusConflict, "模拟故障：资源版本冲突，更新未能提交"},
	{http.StatusTooManyRequests, "模拟故障：请求频率超过限制，触发限流"},
	{http.StatusInternalServerError, "模拟故障：数据库连接池耗尽，查询执行失败"},
	{http.StatusBadGateway, "模拟故障：上游服务连接被重置，网关请求失败"},
	{http.StatusServiceUnavailable, "模拟故障：消息队列不可用，任务暂时无法投递"},
	{http.StatusGatewayTimeout, "模拟故障：上游资源响应超时，未能在截止时间内返回"},
}

func mockError(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	index := rand.IntN(len(mockErrors))
	query, err := url.ParseQuery(request.URL.RawQuery)
	if numbers, specified := query["number"]; specified || err != nil {
		index = -1
		if err == nil && len(numbers) == 1 {
			for candidate := range mockErrors {
				if numbers[0] == strconv.Itoa(1001+candidate) {
					index = candidate
					break
				}
			}
		}
		if index == -1 {
			writeError(writer, request, http.StatusBadRequest, "invalid_mock_number", "number 必须为 1001 到 1010 中的一个编号，且只能指定一次")
			return
		}
	}
	number := 1001 + index
	if logger := requestLoggerFromContext(request.Context()); logger != nil {
		request = request.WithContext(withRequestLogger(request.Context(), logger.With("mock", true, "error_number", number)))
	}
	fault := mockErrors[index]
	writeLoggedErrorNumber(writer, request, fault.status, "mock_"+strconv.Itoa(number), number, fault.message)
}
