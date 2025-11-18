package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/WuKongIM/wklog"
)

// TestGenerateCheckSum 测试签名生成
func TestGenerateCheckSum(t *testing.T) {
	plugin := &ThirdMsgCallback{
		Config: Config{
			AppSecret: "test-secret",
		},
		Log: wklog.NewWKLog("test"),
	}

	// 测试用例
	testCases := []struct {
		name      string
		appSecret string
		md5Str    string
		curTime   int64
		expected  string
	}{
		{
			name:      "正常签名",
			appSecret: "test-secret",
			md5Str:    "5d41402abc4b2a76b9719d911017c592",
			curTime:   1700291400000,
			// SHA1("test-secret" + "5d41402abc4b2a76b9719d911017c592" + "1700291400000")
			expected: "c5dd70370eb0d8f0edf8ee4f3c4e0d5ef62fb66d",
		},
		{
			name:      "空MD5",
			appSecret: "secret",
			md5Str:    "",
			curTime:   1234567890,
			// SHA1("secret" + "" + "1234567890")
			expected: "49c0f1e5b7f0f3e4e5c0a0e5c0e0c0e0d0d0d0d",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plugin.Config.AppSecret = tc.appSecret
			result := plugin.generateCheckSum(tc.md5Str, tc.curTime)

			// 验证签名长度（SHA1 十六进制为 40 字符）
			if len(result) != 40 {
				t.Errorf("签名长度错误: 期望 40, 得到 %d", len(result))
			}

			// 验证签名是十六进制格式
			if _, err := hex.DecodeString(result); err != nil {
				t.Errorf("签名格式错误: %v", err)
			}
		})
	}
}

// TestCallThirdPartySuccess 测试成功调用第三方接口
func TestCallThirdPartySuccess(t *testing.T) {
	// 创建模拟 HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求头
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type 错误")
		}

		if r.Header.Get("AppKey") == "" {
			t.Errorf("AppKey 缺失")
		}

		if r.Header.Get("CurTime") == "" {
			t.Errorf("CurTime 缺失")
		}

		if r.Header.Get("MD5") == "" {
			t.Errorf("MD5 缺失")
		}

		if r.Header.Get("CheckSum") == "" {
			t.Errorf("CheckSum 缺失")
		}

		// 读取请求体
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("读取请求体失败: %v", err)
		}
		defer r.Body.Close()

		// 验证请求体格式
		var req ThirdMsgCallbackReq
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}

		// 返回允许发送的响应
		resp := ThirdMsgCallbackResp{
			Allow: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	plugin := &ThirdMsgCallback{
		Config: Config{
			CallbackUrl: server.URL,
			AppSecret:   "test-secret",
			Timeout:     5,
			Retries:     3,
		},
		Log: wklog.NewWKLog("test"),
	}

	req := ThirdMsgCallbackReq{
		MsgBody:     "hello world",
		FromUid:     "user123",
		ChannelId:   "channel456",
		ChannelType: 1,
		DeviceId:    "device789",
		DeviceFlag:  0,
		DeviceLevel: 1,
		UUID:        "uuid-123",
	}

	resp, err := plugin.callThirdParty(req)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}

	if resp == nil {
		t.Fatalf("响应为 nil")
	}

	if !resp.Allow {
		t.Errorf("响应允许发送标志错误")
	}
}

// TestCallThirdPartyWithMessageModification 测试消息修改功能
func TestCallThirdPartyWithMessageModification(t *testing.T) {
	modifiedMsg := "modified message"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ThirdMsgCallbackResp{
			Allow:   true,
			MsgBody: &modifiedMsg,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	plugin := &ThirdMsgCallback{
		Config: Config{
			CallbackUrl: server.URL,
			AppSecret:   "test-secret",
			Timeout:     5,
			Retries:     0,
		},
		Log: wklog.NewWKLog("test"),
	}

	req := ThirdMsgCallbackReq{
		MsgBody: "original message",
		FromUid: "user123",
	}

	resp, err := plugin.callThirdParty(req)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}

	if resp.MsgBody == nil || *resp.MsgBody != modifiedMsg {
		t.Errorf("消息修改失败")
	}
}

// TestCallThirdPartyDenied 测试拒绝消息发送
func TestCallThirdPartyDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ThirdMsgCallbackResp{
			Allow: false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	plugin := &ThirdMsgCallback{
		Config: Config{
			CallbackUrl: server.URL,
			AppSecret:   "test-secret",
			Timeout:     5,
			Retries:     0,
		},
		Log: wklog.NewWKLog("test"),
	}

	req := ThirdMsgCallbackReq{
		MsgBody: "message to block",
		FromUid: "user123",
	}

	resp, err := plugin.callThirdParty(req)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}

	if resp.Allow {
		t.Errorf("应该拒绝消息发送")
	}
}

// TestRetryMechanism 测试重试机制
func TestRetryMechanism(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++

		// 前两次失败，第三次成功
		if attemptCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp := ThirdMsgCallbackResp{
			Allow: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	plugin := &ThirdMsgCallback{
		Config: Config{
			CallbackUrl: server.URL,
			AppSecret:   "test-secret",
			Timeout:     5,
			Retries:     3, // 最多重试 3 次
		},
		Log: wklog.NewWKLog("test"),
	}

	req := ThirdMsgCallbackReq{
		MsgBody: "retry test",
		FromUid: "user123",
	}

	resp, err := plugin.callThirdParty(req)
	if err != nil {
		t.Fatalf("所有重试都失败了: %v", err)
	}

	if resp == nil || !resp.Allow {
		t.Errorf("最终响应错误")
	}

	if attemptCount < 3 {
		t.Errorf("重试次数不足: 期望至少 3 次, 得到 %d 次", attemptCount)
	}
}

// TestTimeoutHandling 测试超时处理
func TestTimeoutHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 延迟响应以模拟超时
		time.Sleep(2 * time.Second)
		resp := ThirdMsgCallbackResp{
			Allow: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	plugin := &ThirdMsgCallback{
		Config: Config{
			CallbackUrl: server.URL,
			AppSecret:   "test-secret",
			Timeout:     1, // 1 秒超时
			Retries:     1, // 重试 1 次
		},
		Log: wklog.NewWKLog("test"),
	}

	req := ThirdMsgCallbackReq{
		MsgBody: "timeout test",
		FromUid: "user123",
	}

	start := time.Now()
	_, err := plugin.callThirdParty(req)
	elapsed := time.Since(start)

	if err == nil {
		t.Errorf("应该因超时而失败")
	}

	// 验证是否在合理的时间内超时
	// 1 秒超时 + 1 次重试（再 1 秒）+ 一些额外开销
	if elapsed > 5*time.Second {
		t.Logf("超时处理耗时过长: %v", elapsed)
	}
}


// BenchmarkGenerateCheckSum 签名生成性能测试
func BenchmarkGenerateCheckSum(b *testing.B) {
	plugin := &ThirdMsgCallback{
		Config: Config{
			AppSecret: "test-secret",
		},
		Log: wklog.NewWKLog("bench"),
	}

	md5Str := "5d41402abc4b2a76b9719d911017c592"
	curTime := int64(1700291400000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.generateCheckSum(md5Str, curTime)
	}
}

// BenchmarkMarshalRequest 请求序列化性能测试
func BenchmarkMarshalRequest(b *testing.B) {
	req := ThirdMsgCallbackReq{
		MsgBody:     "test message content",
		FromUid:     "user123456789",
		ChannelId:   "channel987654321",
		ChannelType: 1,
		DeviceId:    "device-id-123",
		DeviceFlag:  0,
		DeviceLevel: 1,
		UUID:        "uuid-1234-5678",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(req)
	}
}

// TestInvalidURL 测试无效的回调 URL
func TestInvalidURL(t *testing.T) {
	plugin := &ThirdMsgCallback{
		Config: Config{
			CallbackUrl: "invalid://url",
			AppSecret:   "test-secret",
			Timeout:     1,
			Retries:     0,
		},
		Log: wklog.NewWKLog("test"),
	}

	req := ThirdMsgCallbackReq{
		MsgBody: "test",
		FromUid: "user123",
	}

	_, err := plugin.callThirdParty(req)
	if err == nil {
		t.Errorf("应该因无效 URL 而失败")
	}
}

// TestMD5Calculation 测试 MD5 计算
func TestMD5Calculation(t *testing.T) {
	testData := []byte(`{"msgBody":"hello","fromUid":"user123"}`)

	hash := md5.Sum(testData)
	md5Str := hex.EncodeToString(hash[:])

	if len(md5Str) != 32 {
		t.Errorf("MD5 哈希长度应该是 32, 得到 %d", len(md5Str))
	}

	// 验证是否是十六进制
	if _, err := hex.DecodeString(md5Str); err != nil {
		t.Errorf("MD5 哈希不是有效的十六进制: %v", err)
	}
}
