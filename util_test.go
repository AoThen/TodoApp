package main

import (
	"regexp"
	"testing"
)

// TestRandomStringCryptoQuality 验证 randomString 输出长度、字符集与随机性（密码学源）
func TestRandomStringCryptoQuality(t *testing.T) {
	hexRe := regexp.MustCompile("^[0-9a-f]+$")
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s := randomString(8)
		if len(s) != 8 {
			t.Fatalf("长度应为 8, got %q", s)
		}
		if !hexRe.MatchString(s) {
			t.Fatalf("应为 hex 字符集, got %q", s)
		}
		seen[s] = true
	}
	if len(seen) < 99 {
		t.Errorf("100 次密码学随机生成应几乎不重复, got %d 种", len(seen))
	}
}

// TestRandomStringLengths 验证不同长度请求的输出长度（含奇数长度截断）
func TestRandomStringLengths(t *testing.T) {
	for _, n := range []int{1, 2, 8, 15, 16, 64} {
		if got := randomString(n); len(got) != n {
			t.Errorf("randomString(%d) 长度应为 %d, got %q", n, n, got)
		}
	}
}
