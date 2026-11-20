package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	gorillawebsocket "github.com/gorilla/websocket"

	wsclient "todoapp/internal/websocket"
)

func wsEndpoint() string {
	return strings.Replace(testTS.URL, "http://", "ws://", 1) + "/ws"
}

func loginTokens(t *testing.T, email string) (string, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": testPassword})
	resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Data.AccessToken, out.Data.RefreshToken
}

// waitConnected 轮询等待用户的连接注册/注销生效（Register 走 hub 主循环，异步生效）
func waitConnected(t *testing.T, uid int64, want bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if testWSHub.IsUserConnected(uid) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待用户 %d 在线状态 %v 超时", uid, want)
}

func waitClientCount(t *testing.T, uid int64, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// 全局计数在测试串行执行且此前连接均已关闭的前提下等于该用户连接数
		if testWSHub.GetConnectedClientCount() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待用户 %d 连接数 %d 超时, 当前 %d", uid, want, testWSHub.GetConnectedClientCount())
}

func readWSMessage(t *testing.T, conn *gorillawebsocket.Conn) wsclient.Message {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("读取 WS 消息失败: %v", err)
	}
	var msg wsclient.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("解析 WS 消息失败: %v", err)
	}
	return msg
}

func TestWSAuthChannels(t *testing.T) {
	cleanTables(t)
	uid := int64(userIDByEmail(t, "user@test.local"))

	// 1) Sec-WebSocket-Protocol 子协议通道（todoapp.<token>）
	dialer := &gorillawebsocket.Dialer{Subprotocols: []string{"todoapp." + userTok}}
	conn1, resp, err := dialer.Dial(wsEndpoint(), nil)
	if err != nil {
		t.Fatalf("subprotocol 通道握手失败: %v (resp=%v)", err, resp)
	}
	defer conn1.Close()
	waitConnected(t, uid, true)

	// 2) query 参数通道（向后兼容）
	conn2, _, err := gorillawebsocket.DefaultDialer.Dial(wsEndpoint()+"?token="+url.QueryEscape(userTok), nil)
	if err != nil {
		t.Fatalf("query 通道握手失败: %v", err)
	}
	defer conn2.Close()

	// 3) Authorization header 通道
	header := http.Header{}
	header.Set("Authorization", "Bearer "+userTok)
	conn3, _, err := gorillawebsocket.DefaultDialer.Dial(wsEndpoint(), header)
	if err != nil {
		t.Fatalf("header 通道握手失败: %v", err)
	}
	defer conn3.Close()

	waitClientCount(t, uid, 3)

	// 连接全部关闭后用户离线
	conn1.Close()
	conn2.Close()
	conn3.Close()
	waitConnected(t, uid, false)
}

func TestWSRejectsUnauthenticated(t *testing.T) {
	cleanTables(t)

	// 无 token → 401
	_, resp, err := gorillawebsocket.DefaultDialer.Dial(wsEndpoint(), nil)
	if err == nil {
		t.Fatal("无 token 不应握手成功")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无 token 应返回 401, got %v", resp)
	}

	// 无效 token → 401
	_, resp, err = gorillawebsocket.DefaultDialer.Dial(wsEndpoint()+"?token=invalid-token", nil)
	if err == nil {
		t.Fatal("无效 token 不应握手成功")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无效 token 应返回 401, got %v", resp)
	}

	// refresh token 不能用于 WebSocket（必须是 access 类型）
	_, refresh := loginTokens(t, "user@test.local")
	_, resp, err = gorillawebsocket.DefaultDialer.Dial(wsEndpoint()+"?token="+url.QueryEscape(refresh), nil)
	if err == nil {
		t.Fatal("refresh token 不应握手成功")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refresh token 应返回 401, got %v", resp)
	}

	// 确认无残留 WebSocket 连接
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if testWSHub.GetConnectedClientCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := testWSHub.GetConnectedClientCount(); got != 0 {
		t.Fatalf("应无残留连接, got %d", got)
	}
}

func TestWSNotificationDelivery(t *testing.T) {
	cleanTables(t)
	uid := int64(userIDByEmail(t, "user@test.local"))

	conn, _, err := gorillawebsocket.DefaultDialer.Dial(wsEndpoint()+"?token="+url.QueryEscape(userTok), nil)
	if err != nil {
		t.Fatalf("握手失败: %v", err)
	}
	defer conn.Close()
	waitConnected(t, uid, true)

	msg := wsclient.Message{
		Type:      "notification",
		Data:      map[string]interface{}{"title": "测试通知", "content": "hello"},
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if err := testWSHub.BroadcastToUser(uid, msg); err != nil {
		t.Fatalf("向在线用户广播失败: %v", err)
	}

	got := readWSMessage(t, conn)
	if got.Type != "notification" {
		t.Fatalf("应收到 notification 消息, got %s", got.Type)
	}
	if got.Data["title"] != "测试通知" {
		t.Fatalf("通知内容不符: %v", got.Data)
	}

	// 离线用户广播报错
	if err := testWSHub.BroadcastToUser(999999, msg); err == nil {
		t.Fatal("向离线用户广播应返回错误")
	}
}

func TestWSMultiConnectionFanout(t *testing.T) {
	cleanTables(t)
	uid := int64(userIDByEmail(t, "user@test.local"))

	conn1, _, err := gorillawebsocket.DefaultDialer.Dial(wsEndpoint()+"?token="+url.QueryEscape(userTok), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	conn2, _, err := gorillawebsocket.DefaultDialer.Dial(wsEndpoint()+"?token="+url.QueryEscape(userTok), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()
	waitClientCount(t, uid, 2)

	msg := wsclient.Message{Type: "notification", Data: map[string]interface{}{"title": "多端推送"}, Timestamp: time.Now().Format(time.RFC3339)}
	if err := testWSHub.BroadcastToUser(uid, msg); err != nil {
		t.Fatalf("多连接广播失败: %v", err)
	}
	for i, conn := range []*gorillawebsocket.Conn{conn1, conn2} {
		if got := readWSMessage(t, conn); got.Data["title"] != "多端推送" {
			t.Fatalf("连接 %d 未收到广播: %v", i+1, got)
		}
	}

	// 关闭一条连接后，其余连接仍能收到广播（同用户多连接互不影响）
	conn1.Close()
	waitClientCount(t, uid, 1)
	if err := testWSHub.BroadcastToUser(uid, msg); err != nil {
		t.Fatalf("剩余连接广播失败: %v", err)
	}
	if got := readWSMessage(t, conn2); got.Data["title"] != "多端推送" {
		t.Fatalf("剩余连接未收到广播: %v", got)
	}
}
