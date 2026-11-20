package config

import (
	"testing"
	"time"
)

func TestEnvDuration(t *testing.T) {
	t.Run("未设置返回默认值", func(t *testing.T) {
		if got := EnvDuration("CONFIG_TEST_DURATION_UNSET", 15*time.Minute); got != 15*time.Minute {
			t.Errorf("got %v, want 15m", got)
		}
	})
	t.Run("合法值覆盖", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_DURATION", "45m")
		if got := EnvDuration("CONFIG_TEST_DURATION", 15*time.Minute); got != 45*time.Minute {
			t.Errorf("got %v, want 45m", got)
		}
	})
	t.Run("非法值回退默认", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_DURATION", "abc")
		if got := EnvDuration("CONFIG_TEST_DURATION", 15*time.Minute); got != 15*time.Minute {
			t.Errorf("got %v, want 15m", got)
		}
	})
	t.Run("非正值回退默认", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_DURATION", "-5m")
		if got := EnvDuration("CONFIG_TEST_DURATION", 15*time.Minute); got != 15*time.Minute {
			t.Errorf("got %v, want 15m", got)
		}
	})
	t.Run("小时与天单位", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_DURATION", "168h")
		if got := EnvDuration("CONFIG_TEST_DURATION", time.Hour); got != 168*time.Hour {
			t.Errorf("got %v, want 168h", got)
		}
	})
}

func TestEnvInt(t *testing.T) {
	t.Run("未设置返回默认值", func(t *testing.T) {
		if got := EnvInt("CONFIG_TEST_INT_UNSET", 5); got != 5 {
			t.Errorf("got %d, want 5", got)
		}
	})
	t.Run("合法值覆盖", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_INT", "10")
		if got := EnvInt("CONFIG_TEST_INT", 5); got != 10 {
			t.Errorf("got %d, want 10", got)
		}
	})
	t.Run("非法值回退默认", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_INT", "abc")
		if got := EnvInt("CONFIG_TEST_INT", 5); got != 5 {
			t.Errorf("got %d, want 5", got)
		}
	})
	t.Run("非正值回退默认", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_INT", "0")
		if got := EnvInt("CONFIG_TEST_INT", 5); got != 5 {
			t.Errorf("got %d, want 5", got)
		}
	})
}

func TestEnvBool(t *testing.T) {
	t.Run("未设置返回默认值", func(t *testing.T) {
		if got := EnvBool("CONFIG_TEST_BOOL_UNSET", true); !got {
			t.Error("got false, want true")
		}
	})
	t.Run("true与1", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_BOOL", "TRUE")
		if got := EnvBool("CONFIG_TEST_BOOL", false); !got {
			t.Error("got false, want true")
		}
		t.Setenv("CONFIG_TEST_BOOL", "1")
		if got := EnvBool("CONFIG_TEST_BOOL", false); !got {
			t.Error("got false, want true")
		}
	})
	t.Run("false与0", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_BOOL", "false")
		if got := EnvBool("CONFIG_TEST_BOOL", true); got {
			t.Error("got true, want false")
		}
		t.Setenv("CONFIG_TEST_BOOL", "0")
		if got := EnvBool("CONFIG_TEST_BOOL", true); got {
			t.Error("got true, want false")
		}
	})
	t.Run("非法值回退默认", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_BOOL", "yes")
		if got := EnvBool("CONFIG_TEST_BOOL", true); !got {
			t.Error("got false, want true")
		}
	})
}
