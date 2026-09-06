package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/liaogx/douyin-mcp/douyin"
	"io"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, result any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}

func writeResult(w http.ResponseWriter, result any, err error) {
	if err == nil {
		writeJSON(w, 200, result)
		return
	}
	status, code := 500, "operation_failed"
	var p *douyin.Error
	if errors.As(err, &p) {
		status, code = p.Status, p.Code
	} else if errors.Is(err, context.DeadlineExceeded) {
		status, code = 504, "timeout"
	} else if errors.Is(err, context.Canceled) {
		status, code = 408, "cancelled"
	}
	writeJSON(w, status, map[string]any{"error": code, "message": err.Error(), "result": result})
}

func decodeJSON(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return &douyin.Error{Code: "invalid_json", Message: "JSON 参数错误或包含不支持的字段", Status: 400}
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return &douyin.Error{Code: "invalid_json", Message: "请求只能包含一个 JSON 对象", Status: 400}
	}
	return nil
}

func SetupRoutes(service Operations, mcpHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "service": "douyin-mcp", "version": version})
	})
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("GET /api/v1/login/status", func(w http.ResponseWriter, r *http.Request) {
		result, err := loginForSurface(r.Context(), service, douyin.LoginRequest{Surface: r.URL.Query().Get("surface")}, false)
		writeResult(w, result, err)
	})
	mux.HandleFunc("POST /api/v1/login/qrcode", func(w http.ResponseWriter, r *http.Request) {
		var args douyin.LoginRequest
		if r.ContentLength != 0 {
			if err := decodeJSON(r, &args); err != nil {
				writeResult(w, nil, err)
				return
			}
		}
		result, err := loginForSurface(r.Context(), service, args, true)
		writeResult(w, result, err)
	})
	mux.HandleFunc("DELETE /api/v1/cookies", func(w http.ResponseWriter, r *http.Request) {
		err := service.DeleteCookies(r.Context())
		if err != nil {
			writeResult(w, nil, err)
			return
		}
		writeResult(w, map[string]string{"message": "本地凭证与专用浏览器会话已清除；不代表撤销其他设备或平台端会话"}, err)
	})
	for path, call := range map[string]func(context.Context, *douyin.PublishRequest) (*douyin.PublishResult, error){"/api/v1/publish/video": service.PublishVideo, "/api/v1/publish/image-text": service.PublishImageText} {
		mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
			var req douyin.PublishRequest
			if err := decodeJSON(r, &req); err != nil {
				writeResult(w, nil, err)
				return
			}
			result, err := call(r.Context(), &req)
			writeResult(w, result, err)
		})
	}
	if web, ok := service.(WebOperations); ok {
		addWebRoutes(mux, web)
	}
	return mux
}
