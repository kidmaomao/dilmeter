package main

import (
	"strings"
	"testing"
)

func TestSanitizeLocalTTSText(t *testing.T) {
	if got := sanitizeLocalTTSText("  音乐\n要\t结束了\x00  "); got != "音乐 要 结束了" {
		t.Fatalf("sanitizeLocalTTSText() = %q", got)
	}
	if got := localTTSDisplayName("这是一个用于测试本地语音文件名称长度限制的很长提示文本"); !strings.Contains(got, "…") || !strings.HasSuffix(got, ".wav") {
		t.Fatalf("local TTS display name was not safely shortened: %q", got)
	}
}
