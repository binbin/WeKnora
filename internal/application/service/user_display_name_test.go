package service

import "testing"

func TestValidateChineseDisplayName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "two chars ok", input: "张三", wantErr: false},
		{name: "twenty ok", input: "一二三四五六七八九十一二三四五六七八九十", wantErr: false},
		{name: "one char", input: "张", wantErr: true},
		{name: "twenty one", input: "一二三四五六七八九十一二三四五六七八九十多", wantErr: true},
		{name: "latin", input: "Hank", wantErr: true},
		{name: "mixed", input: "张三A", wantErr: true},
		{name: "digits", input: "张三1", wantErr: true},
		{name: "empty", input: "  ", wantErr: true},
	}
	for _, testCase := range cases {
		err := ValidateChineseDisplayName(testCase.input)
		if testCase.wantErr && err == nil {
			t.Fatalf("%s: want error", testCase.name)
		}
		if !testCase.wantErr && err != nil {
			t.Fatalf("%s: unexpected %v", testCase.name, err)
		}
	}
}
