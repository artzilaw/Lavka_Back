package blacklist

import (
	"testing"
	"time"
)

func TestInMemory_AddCheckAndCleanup(t *testing.T) {
	bl := New(10 * time.Millisecond)
	defer bl.Stop()

	// 1. Добавляем валидный токен
	bl.Add("token-1", time.Now().Add(1*time.Hour))
	if !bl.Check("token-1") {
		t.Error("expected token-1 to be in blacklist")
	}
	if bl.Check("unknown-token") {
		t.Error("expected unknown-token to NOT be in blacklist")
	}

	// 2. Добавляем токен, который уже протух, и ждём срабатывания sweeper'а
	bl.Add("expired-token", time.Now().Add(-1*time.Second))
	time.Sleep(30 * time.Millisecond)

	if bl.Check("expired-token") {
		t.Error("expected expired-token to be cleaned up by sweeper")
	}
}
