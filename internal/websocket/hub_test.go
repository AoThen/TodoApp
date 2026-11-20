package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(h *Hub, userID int64) *Client {
	return NewClient(h, nil, userID, "user@test.local", false)
}

func waitHubCount(t *testing.T, h *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.GetConnectedClientCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待在线连接数 %d 超时, 当前 %d", want, h.GetConnectedClientCount())
}

func TestHubRegisterAndBroadcastToUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	c1 := newTestClient(hub, 1)
	c2 := newTestClient(hub, 1) // 同一用户第二条连接（多设备/多标签页）
	c3 := newTestClient(hub, 2)

	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)
	waitHubCount(t, hub, 3)

	if !hub.IsUserConnected(1) || !hub.IsUserConnected(2) {
		t.Fatal("用户 1/2 应在线")
	}
	if hub.IsUserConnected(3) {
		t.Fatal("用户 3 不应在线")
	}

	msg := Message{Type: "notification", Timestamp: "t"}
	if err := hub.BroadcastToUser(1, msg); err != nil {
		t.Fatalf("向用户 1 广播失败: %v", err)
	}
	select {
	case got := <-c1.send:
		if len(got) == 0 {
			t.Fatal("连接 1 收到空消息")
		}
	default:
		t.Fatal("连接 1 未收到广播")
	}
	select {
	case got := <-c2.send:
		if len(got) == 0 {
			t.Fatal("连接 2 收到空消息")
		}
	default:
		t.Fatal("连接 2 未收到广播")
	}
	select {
	case <-c3.send:
		t.Fatal("用户 2 的连接不应收到发给用户 1 的消息")
	default:
	}

	if err := hub.BroadcastToUser(999, msg); err == nil {
		t.Fatal("向离线用户广播应返回错误")
	}

	hub.Unregister(c1)
	hub.Unregister(c2)
	hub.Unregister(c3)
	waitHubCount(t, hub, 0)
}

func TestHubBroadcastRemovesFullBufferClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	c := newTestClient(hub, 7)
	hub.Register(c)
	waitHubCount(t, hub, 1)

	// 塞满发送缓冲（容量 256），模拟消费方卡死的连接
	for i := 0; i < cap(c.send); i++ {
		c.send <- []byte("x")
	}

	if err := hub.Broadcast(Message{Type: "notification"}); err != nil {
		t.Fatalf("全局广播不应报错: %v", err)
	}
	// 缓冲已满的连接应在广播后被剔除并关闭
	waitHubCount(t, hub, 0)
}

func TestHubRemoveLockedIdempotent(t *testing.T) {
	hub := NewHub()
	c := newTestClient(hub, 5)
	hub.clients[5] = map[*Client]struct{}{c: {}}

	hub.removeLocked(c)
	if hub.IsUserConnected(5) {
		t.Fatal("移除后用户不应在线")
	}

	// 重复移除不应 panic 或重复 close
	hub.removeLocked(c)
	hub.removeLocked(c)
}

func TestSafeSend(t *testing.T) {
	// 正常投递
	ch := make(chan []byte, 1)
	if !safeSend(ch, []byte("a")) {
		t.Fatal("正常投递应返回 true")
	}
	// 缓冲已满
	if safeSend(ch, []byte("b")) {
		t.Fatal("缓冲已满应返回 false")
	}
	// 通道已关闭：应恢复 panic 并返回 false，而非崩溃
	closed := make(chan []byte, 1)
	close(closed)
	if safeSend(closed, []byte("c")) {
		t.Fatal("向已关闭通道投递应返回 false")
	}
}

func TestSendMessageOnClosedChannelDoesNotPanic(t *testing.T) {
	hub := NewHub()
	c := newTestClient(hub, 9)
	close(c.send)
	// send 通道已关闭：safeSend 恢复 panic，SendMessage 静默丢弃
	if err := c.SendMessage(Message{Type: "ping"}); err != nil {
		t.Fatalf("SendMessage 不应返回错误: %v", err)
	}
}

func TestCheckOrigin(t *testing.T) {
	t.Cleanup(func() { SetAllowedOrigins(nil) })

	// 未配置白名单：放行（仅限开发）
	SetAllowedOrigins(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	if !Upgrader.CheckOrigin(req) {
		t.Fatal("未配置白名单时应放行")
	}

	// 配置白名单：精确匹配（大小写不敏感）
	SetAllowedOrigins([]string{"https://app.example.com"})
	req.Header.Set("Origin", "https://APP.example.com")
	if !Upgrader.CheckOrigin(req) {
		t.Fatal("白名单内的 Origin 应放行")
	}

	// 不在白名单
	req.Header.Set("Origin", "https://evil.example.com")
	if Upgrader.CheckOrigin(req) {
		t.Fatal("白名单外的 Origin 应拒绝")
	}

	// 空 Origin（非浏览器客户端）拒绝
	req.Header.Del("Origin")
	if Upgrader.CheckOrigin(req) {
		t.Fatal("白名单模式下空 Origin 应拒绝")
	}
}
