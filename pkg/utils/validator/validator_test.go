package validator

import (
	"errors"
	"net/http"
	"testing"

	"gacha_srv/pkg/utils/errorx"
)

type testReq struct {
	Username string `json:"username" validate:"required,min=3,max=32,name"`
	Password string `json:"password" validate:"required,min=6,max=32,passwordFormat"`
	Amount   string `json:"amount" validate:"required,dgt=0"`
	BoxId    int64  `json:"boxId" validate:"gte=1"`
}

func TestValidate(t *testing.T) {
	v := New()

	valid := testReq{Username: "alice", Password: "pass123456", Amount: "1.00", BoxId: 1}

	tests := []struct {
		name    string
		req     testReq
		wantMsg string
	}{
		{name: "ok", req: valid},
		{name: "required", req: testReq{Username: "alice", Password: "pass123456", BoxId: 1},
			wantMsg: "金额不能为空"},
		{name: "min-length", req: testReq{Username: "ab", Password: "pass123456", Amount: "1.00", BoxId: 1},
			wantMsg: "用户名长度不能少于3"},
		{name: "max-length", req: testReq{Username: "abcdefghijklmnopqrstuvwxyz0123456", Password: "pass123456", Amount: "1.00", BoxId: 1},
			wantMsg: "用户名长度不能超过32"},
		{name: "name-format", req: testReq{Username: "ab c", Password: "pass123456", Amount: "1.00", BoxId: 1},
			wantMsg: "用户名格式不正确，只能包含中文、字母、数字和特殊字符"},
		{name: "password-format", req: testReq{Username: "alice", Password: "pass word", Amount: "1.00", BoxId: 1},
			wantMsg: "密码格式不正确，只能包含中文、字母、数字和特殊字符"},
		{name: "gte-numeric", req: testReq{Username: "alice", Password: "pass123456", Amount: "1.00"},
			wantMsg: "宝箱ID必须大于或等于1"},
		{name: "dgt-amount", req: testReq{Username: "alice", Password: "pass123456", Amount: "0", BoxId: 1},
			wantMsg: "金额必须大于0"},
		{name: "amount-not-number", req: testReq{Username: "alice", Password: "pass123456", Amount: "abc", BoxId: 1},
			wantMsg: "金额必须大于0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(&http.Request{}, &tt.req)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("expect no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expect validation error, got nil")
			}
			var ce *errorx.CodeError
			if !errors.As(err, &ce) {
				t.Fatalf("expect CodeError, got %T: %v", err, err)
			}
			if ce.Code != errorx.CodeParamError {
				t.Fatalf("expect code 400, got %d", ce.Code)
			}
			if ce.Msg != tt.wantMsg {
				t.Fatalf("expect msg %q, got %q", tt.wantMsg, ce.Msg)
			}
		})
	}
}

func TestValidateAll(t *testing.T) {
	v := New()
	req := testReq{Username: "", Password: "x", Amount: "0"}
	errs := v.ValidateAll(&req)
	if len(errs) == 0 {
		t.Fatal("expect errors, got none")
	}
	for field, msg := range errs {
		if field == "" || msg == "" {
			t.Fatalf("expect non-empty field/msg, got %q: %q", field, msg)
		}
	}
	if errs["用户名"] == "" || errs["密码"] == "" || errs["金额"] == "" {
		t.Fatalf("expect friendly names 用户名/密码/金额, got %v", errs)
	}
}

func TestSetActive(t *testing.T) {
	v := New()
	v.SetActive(false)
	if err := v.Validate(&http.Request{}, &testReq{}); err != nil {
		t.Fatalf("expect no error when inactive, got %v", err)
	}
}
