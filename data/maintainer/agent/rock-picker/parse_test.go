package main

import (
	"strings"
	"testing"
)

func TestParseRockCount(t *testing.T) {
	cases := []struct {
		raw     string
		want    int
		wantOK  bool
		comment string
	}{
		{"9,428", 9428, true, "游戏内半角逗号分位"},
		{"9428", 9428, true, "无分隔符"},
		{"12,008", 12008, true, "五位数"},
		{"16,724", 16724, true, "五位数"},
		{"9 428", 9428, true, "空格分隔"},
		{"9，428", 9428, true, "全角逗号"},
		{"９４２８", 9428, true, "全角数字"},
		{"12,O08", 12008, true, "O→0 混淆"},
		{"1O,722", 10722, true, "1O→10 混淆"},
		{"9,42B", 9428, true, "B→8 混淆"},
		{"9,428", 9428, true, "纯数字"},
		{"  9,428 ", 9428, true, "首尾空白"},
		{"9.428", 9428, true, "点号分隔"},
		{"0", 0, true, "零值合法"},
		{"", 0, false, "空串 → 读不到"},
		{"   ", 0, false, "全空白 → 读不到"},
		{"火之洞穴", 0, false, "OCR 读成了别的文字"},
		{"9,4a8", 0, false, "夹杂无法纠正的字符"},
		{"123456789", 0, false, "位数超限 → 疑似误读"},
		{"12345678", 0, false, "八位数 → 超出上限"},
	}
	for _, c := range cases {
		got, cleaned, ok := parseRockCount(c.raw)
		if ok != c.wantOK {
			t.Errorf("parseRockCount(%q) ok = %v, want %v (%s)", c.raw, ok, c.wantOK, c.comment)
			continue
		}
		if !ok {
			continue
		}
		if got != c.want {
			t.Errorf("parseRockCount(%q) = %d, want %d (%s)", c.raw, got, c.want, c.comment)
		}
		if strings.TrimSpace(cleaned) == "" {
			t.Errorf("parseRockCount(%q) 清洗结果为空", c.raw)
		}
	}
}

func TestPickLeast(t *testing.T) {
	cases := []struct {
		counts []int
		want   int
	}{
		{[]int{13010, 9177, 12008, 10722, 16724}, 1},
		{[]int{3, 2, 1, 4, 5}, 2},
		{[]int{5, 4, 3, 2, 1}, 4},
		{[]int{7, 7, 7, 7, 7}, 0}, // 并列 → 取最早的一行
		{[]int{9, 9, 3, 3, 8}, 2}, // 并列最小值 → 取最早的一行
		{[]int{0, 1, 2, 3, 4}, 0},
	}
	for _, c := range cases {
		got, err := pickLeast(c.counts)
		if err != nil {
			t.Fatalf("pickLeast(%v) 返回错误: %v", c.counts, err)
		}
		if got != c.want {
			t.Errorf("pickLeast(%v) = %d, want %d", c.counts, got, c.want)
		}
	}
	if _, err := pickLeast(nil); err == nil {
		t.Error("pickLeast(nil) 应当返回错误")
	}
}

func TestFormatCounts(t *testing.T) {
	labels := []string{"火", "水", "风", "光", "暗"}
	counts := []int{13010, 9177, 12008, 10722, 16724}
	got := formatCounts(labels, counts)
	want := "火=13010 水=9177 风=12008 光=10722 暗=16724"
	if got != want {
		t.Errorf("formatCounts = %q, want %q", got, want)
	}
}
