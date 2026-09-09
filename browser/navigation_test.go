package browser

import (
	"errors"
	"testing"
)

func TestTransientNavigationErrorsAreNarrowlyClassified(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "closed", err: errors.New("navigation failed: net::ERR_CONNECTION_CLOSED"), want: true},
		{name: "reset", err: errors.New("net::ERR_CONNECTION_RESET"), want: true},
		{name: "timeout", err: errors.New("net::ERR_TIMED_OUT"), want: true},
		{name: "closed cdp", err: errors.New("write tcp 127.0.0.1:1->127.0.0.1:2: use of closed network connection"), want: false},
		{name: "selector", err: errors.New("元素不可见"), want: false},
		{name: "challenge", err: errors.New("请完成安全验证"), want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isTransientNavigationError(test.err); got != test.want {
				t.Fatalf("isTransientNavigationError(%q) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

func TestTransientBrowserConnectionErrorsIncludeClosedCDP(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "closed cdp", err: errors.New("write tcp: use of closed network connection"), want: true},
		{name: "websocket", err: errors.New("websocket is closed"), want: true},
		{name: "selector", err: errors.New("元素不可见"), want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isTransientBrowserConnectionError(test.err); got != test.want {
				t.Fatalf("isTransientBrowserConnectionError(%q) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}
