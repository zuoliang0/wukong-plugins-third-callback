package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	wkproto "github.com/WuKongIM/WuKongIMGoProto"
	"github.com/WuKongIM/go-pdk/pdk"
	"github.com/WuKongIM/wklog"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

/*
*
 */
var PluginNo = "wk.plugin.third.msg.callback" // 插件编号
var Version = "0.0.1"                         // 插件版本
var Priority = int32(1)                       // 插件优先级
// 定义插件的配置结构体
type Config struct {
	CallbackUrl         string `json:"name" label:"第三方接口URL"`          // json为配置项名称，label为WuKongIM后台显示的配置项名称
	AppSecret           string `json:"app_secret" label:"签名密钥"`        // json为配置项名称，label为WuKongIM后台显示的配置项名称
	Timeout             int    `json:"timeout" label:"请求超时时间(秒)"`      // json为配置项名称，label为WuKongIM后台显示的配置项名称
	TimeoutSend         bool   `json:"timeout_send" label:"超时后是否允许发送"` // json为配置项名称，label为WuKongIM后台显示的配置项名称
	Retries             int    `json:"retries" label:"重试次数"`           // json为配置项名称，label为WuKongIM后台显示的配置项名称
	CircuitBreakerLimit int    `json:"circuit_breaker_limit" label:"熔断阈值(连续失败次数)"`
	CircuitBreakerReset int    `json:"circuit_breaker_reset" label:"熔断重置时间(秒)"`
}

// 插件结构体
type ThirdMsgCallback struct {
	Config Config // 插件的配置，名字必须为Config, 声明了以后，可以在WuKongIM后台配置，WuKongIM后台配置后会自动填充Config的数据，从而在函数里可以直接使用Config里的属性
	wklog.Log

	// HTTP 连接池（性能优化）
	httpClient *http.Client
	httpMutex  sync.Mutex

	// 熔断器状态（可靠性提升）
	failureCount     atomic.Int32
	lastFailureTime  time.Time
	circuitBreakerMu sync.Mutex
}

// 创建一个插件对象
func New() interface{} {
	// 创建 HTTP 客户端（包含连接池）
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second, // 全局超时，会被 Config.Timeout 覆盖
	}

	return &ThirdMsgCallback{
		// 初始化日志
		Log: wklog.NewWKLog("third-msg-callback"),
		Config: Config{
			CallbackUrl:         "http://localhost:1234", // 默认URL
			AppSecret:           "1234",                  // 默认密钥
			Timeout:             5,                       // 默认请求超时时间5秒
			TimeoutSend:         false,                   // 默认超时后不允许发送
			Retries:             3,                       // 默认重试3次
			CircuitBreakerLimit: 10,                      // 默认熔断阈值：连续失败10次
			CircuitBreakerReset: 60,                      // 默认熔断重置时间：60秒
		},
		httpClient:      httpClient,
		lastFailureTime: time.Now(),
	}
}

type ThirdMsgCallbackReq struct {
	MsgBody     string             `json:"msgBody"`
	FromUid     string             `json:"fromUid"`
	ChannelId   string             `json:"channelId"`
	ChannelType uint32             `json:"channelType"`
	DeviceId    string             `json:"deviceId"`
	DeviceFlag  wkproto.DeviceFlag `json:"deviceFlag"`  // APP 0   WEB = 1 PC = 2 SYSTEM = 99
	DeviceLevel uint32             `json:"deviceLevel"` //0从设备 1主设备
	UUID        string             `json:"uuid"`        //消息唯一标识 仅用于日志跟踪
}
type ThirdMsgCallbackResp struct {
	Allow   bool    `json:"allow"`
	MsgBody *string `json:"msgBody,omitempty"` //非必传，允许修改消息内容base64格式
}

func (r *ThirdMsgCallback) Send(c *pdk.Context) {
	// 获取设备信息（从 SendPacket.Conn 中获取）
	var deviceId string
	var deviceFlag uint32
	var deviceLevel uint32

	if c.SendPacket.Conn != nil {
		deviceId = c.SendPacket.Conn.DeviceId
		deviceFlag = c.SendPacket.Conn.DeviceFlag
		deviceLevel = c.SendPacket.Conn.DeviceLevel
	}

	// 构造回调请求
	//base64
	msgbody := base64.StdEncoding.EncodeToString(c.SendPacket.Payload)
	req := ThirdMsgCallbackReq{
		MsgBody:     msgbody,
		FromUid:     c.SendPacket.FromUid,
		ChannelId:   c.SendPacket.ChannelId,
		ChannelType: c.SendPacket.ChannelType,
		DeviceId:    deviceId,
		DeviceFlag:  wkproto.DeviceFlag(deviceFlag),
		DeviceLevel: deviceLevel,
		UUID:        uuid.New().String(),
	}

	// 发送回调请求（带重试）
	resp, err := r.callThirdParty(req)
	if err != nil {
		r.Error("Failed to call third-party API", zap.Error(err))
		// 根据 TimeoutSend 配置决定是否允许发送
		if r.Config.TimeoutSend {
			c.SendPacket.Reason = uint32(wkproto.ReasonSuccess)
		} else {
			c.SendPacket.Reason = uint32(wkproto.ReasonNotAllowSend)
		}
		return
	}

	// 根据响应设置是否允许发送
	if resp.Allow {
		c.SendPacket.Reason = uint32(wkproto.ReasonSuccess)
		r.Info("Message allowed to send", zap.String("fromUid", req.FromUid))
	} else {
		c.SendPacket.Reason = uint32(wkproto.ReasonNotAllowSend)
		r.Info("Message blocked by third-party", zap.String("fromUid", req.FromUid))
	}

	// 如果响应包含修改后的消息，更新消息内容
	if resp.MsgBody != nil && *resp.MsgBody != "" {
		// 解码base64
		decodedMsgBody, err := base64.StdEncoding.DecodeString(*resp.MsgBody)
		if err != nil {
			r.Error("Failed to decode modified message body", zap.Error(err))
			return
		}
		c.SendPacket.Payload = decodedMsgBody
		r.Info("Message body updated", zap.String("fromUid", req.FromUid))
	}
}

// checkCircuitBreaker 检查熔断器状态
func (r *ThirdMsgCallback) checkCircuitBreaker() bool {
	r.circuitBreakerMu.Lock()
	defer r.circuitBreakerMu.Unlock()

	failCount := r.failureCount.Load()

	// 检查是否达到熔断阈值
	if int(failCount) >= r.Config.CircuitBreakerLimit {
		// 检查是否可以尝试恢复
		elapsed := time.Since(r.lastFailureTime).Seconds()
		resetTime := time.Duration(r.Config.CircuitBreakerReset) * time.Second

		if elapsed >= resetTime.Seconds() {
			// 尝试恢复：重置失败计数
			r.failureCount.Store(0)
			r.Info("Circuit breaker recovering, resetting failure count",
				zap.Int32("previousCount", failCount),
				zap.Float64("elapsedSeconds", elapsed))
			return true
		}

		// 仍在熔断状态
		r.Warn("Circuit breaker is open, rejecting request",
			zap.Int32("failureCount", failCount),
			zap.Int("limit", r.Config.CircuitBreakerLimit),
			zap.Float64("secondsUntilReset", resetTime.Seconds()-elapsed))
		return false
	}

	return true
}

// recordSuccess 记录成功，重置失败计数
func (r *ThirdMsgCallback) recordSuccess() {
	r.circuitBreakerMu.Lock()
	defer r.circuitBreakerMu.Unlock()

	if r.failureCount.Load() > 0 {
		r.Info("Request succeeded, resetting failure count",
			zap.Int32("previousCount", r.failureCount.Load()))
		r.failureCount.Store(0)
	}
}

// recordFailure 记录失败，增加失败计数
func (r *ThirdMsgCallback) recordFailure() {
	r.circuitBreakerMu.Lock()
	defer r.circuitBreakerMu.Unlock()

	count := r.failureCount.Add(1)
	r.lastFailureTime = time.Now()

	r.Error("Request failed, incrementing failure count",
		zap.Int32("failureCount", count),
		zap.Int("circuitBreakerLimit", r.Config.CircuitBreakerLimit))
}

// calculateRetryDelay 计算重试延迟（指数退避）
// 算法：初始延迟 1 秒，每次重试翻倍，最多延迟 32 秒
func calculateRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// 指数退避：1, 2, 4, 8, 16, 32...
	delaySeconds := 1 << uint(attempt-1) // 2^(attempt-1)

	// 最多延迟 32 秒
	if delaySeconds > 32 {
		delaySeconds = 32
	}

	return time.Duration(delaySeconds) * time.Second
}

// callThirdParty 调用第三方接口，带重试机制
func (r *ThirdMsgCallback) callThirdParty(req ThirdMsgCallbackReq) (*ThirdMsgCallbackResp, error) {
	// 检查熔断器状态
	if !r.checkCircuitBreaker() {
		return nil, fmt.Errorf("circuit breaker is open, rejecting request")
	}

	// 将请求序列化为 JSON
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 计算签名
	curTime := time.Now().UnixMilli()
	md5Hash := md5.Sum([]byte(req.MsgBody))
	md5Str := hex.EncodeToString(md5Hash[:])
	checkSum := r.generateCheckSum(md5Str, curTime)

	r.Info("Signature info",
		zap.String("md5", md5Str),
		zap.Int64("curTime", curTime),
		zap.String("checkSum", checkSum))

	// 重试逻辑（带指数退避延迟）
	var lastErr error
	for attempt := 0; attempt <= r.Config.Retries; attempt++ {
		if attempt > 0 {
			// 计算延迟（指数退避）
			delay := calculateRetryDelay(attempt)
			r.Info("Retrying third-party call",
				zap.Int("attempt", attempt),
				zap.String("url", r.Config.CallbackUrl),
				zap.Duration("delayBeforeRetry", delay))
			time.Sleep(delay)
		}

		resp, err := r.doRequest(reqBodyBytes, md5Str, curTime, checkSum)
		if err == nil {
			// 成功，记录成功状态
			r.recordSuccess()
			return resp, nil
		}

		lastErr = err
		r.Error("Third-party call failed",
			zap.Int("attempt", attempt),
			zap.String("url", r.Config.CallbackUrl),
			zap.Error(err))

		// 每次失败都记录失败状态（用于熔断）
		r.recordFailure()
	}

	return nil, fmt.Errorf("all retry attempts failed: %w", lastErr)
}

// doRequest 执行单次 HTTP 请求（使用全局 HTTP 连接池）
func (r *ThirdMsgCallback) doRequest(reqBodyBytes []byte, md5Str string, curTime int64, checkSum string) (*ThirdMsgCallbackResp, error) {
	// 创建请求
	httpReq, err := http.NewRequest("POST", r.Config.CallbackUrl, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("AppKey", r.Config.AppSecret)
	httpReq.Header.Set("CurTime", fmt.Sprintf("%d", curTime))
	httpReq.Header.Set("MD5", md5Str)
	httpReq.Header.Set("CheckSum", checkSum)

	// 创建带超时的 context，覆盖全局超时
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.Config.Timeout)*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	// 使用全局 HTTP 客户端发送请求（自动复用连接）
	r.httpMutex.Lock()
	httpResp, err := r.httpClient.Do(httpReq)
	r.httpMutex.Unlock()

	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// 读取响应体
	respBodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 检查 HTTP 状态码
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", httpResp.StatusCode, string(respBodyBytes))
	}

	// 解析响应
	var resp ThirdMsgCallbackResp
	if err := json.Unmarshal(respBodyBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// generateCheckSum 生成校验签名
// CheckSum = SHA1(AppSecret + MD5 + CurTime)
func (r *ThirdMsgCallback) generateCheckSum(md5Str string, curTime int64) string {
	curTimeStr := fmt.Sprintf("%d", curTime)
	data := r.Config.AppSecret + md5Str + curTimeStr
	hash := sha1.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// main 启动插件服务
func main() {
	err := pdk.RunServer(New, PluginNo, pdk.WithVersion(Version), pdk.WithPriority(Priority))
	if err != nil {
		panic(err)
	}
}
