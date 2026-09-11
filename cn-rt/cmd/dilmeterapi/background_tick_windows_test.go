//go:build windows

package main

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEnsureBackgroundWebViewTimersIsIdempotent(t *testing.T) {
	const key = "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS"
	previous, present := os.LookupEnv(key)
	t.Cleanup(func() {
		if present {
			_ = os.Setenv(key, previous)
		} else {
			_ = os.Unsetenv(key)
		}
	})
	if err := os.Setenv(key, "--existing-argument"); err != nil {
		t.Fatal(err)
	}

	ensureBackgroundWebViewTimers()
	ensureBackgroundWebViewTimers()
	value := os.Getenv(key)
	for _, argument := range []string{
		"--disable-background-timer-throttling",
		"--disable-renderer-backgrounding",
		"--disable-backgrounding-occluded-windows",
		"--disable-features=IntensiveWakeUpThrottling",
	} {
		if count := strings.Count(value, argument); count != 1 {
			t.Fatalf("expected %q exactly once, got %d in %q", argument, count, value)
		}
	}
}

func TestNativeTickUsesSingleDispatcherAndStops(t *testing.T) {
	var mu sync.Mutex
	dispatchCount := 0
	evaluationCount := [3]int{}
	dispatch := func(callback func()) {
		mu.Lock()
		dispatchCount++
		mu.Unlock()
		callback()
	}
	evaluators := make([]func(string), len(evaluationCount))
	for index := range evaluationCount {
		index := index
		evaluators[index] = func(script string) {
			if !strings.Contains(script, "dilmeter-native-tick") {
				t.Errorf("unexpected heartbeat script: %q", script)
			}
			if !strings.Contains(script, "Date.now()") || !strings.Contains(script, "__dilmeterNativeTickAt") {
				t.Errorf("heartbeat must coalesce queued ticks with the renderer clock: %q", script)
			}
			mu.Lock()
			evaluationCount[index]++
			mu.Unlock()
		}
	}

	stop := startNativeTickLoop(context.Background(), 5*time.Millisecond, dispatch, evaluators...)
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		ready := dispatchCount >= 2
		mu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	stop()
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if dispatchCount < 2 {
		t.Fatalf("native clock did not dispatch enough ticks: %d", dispatchCount)
	}
	for index, count := range evaluationCount {
		if count != dispatchCount {
			t.Fatalf("evaluator %d ran %d times for %d dispatched ticks", index, count, dispatchCount)
		}
	}
}
