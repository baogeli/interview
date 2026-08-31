package utils

import "testing"

// 测试正常情况
func TestAdd_Success(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}
}

// 测试边界或特殊情况（如负数）
func TestAdd_NegativeNumbers(t *testing.T) {
	result := Add(-1, -1)
	if result != -2 {
		t.Errorf("Expected -2, got %d", result)
	}
}
