package generator

import (
	"testing"
)

func TestCentsToDigits(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected [12]int
	}{
		{
			name:     "零值",
			cents:    0,
			expected: [12]int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:     "正整数 1234.56 元",
			cents:    123456,
			expected: [12]int{0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6},
		},
		{
			name:     "大额 9999999999.99 元",
			cents:    999999999999,
			expected: [12]int{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9},
		},
		{
			name:     "负数 -1234.56 元",
			cents:    -123456,
			expected: [12]int{0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6},
		},
		{
			name:     "小金额 0.01 元",
			cents:    1,
			expected: [12]int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		},
		{
			name:     "整百元 100.00 元",
			cents:    10000,
			expected: [12]int{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0},
		},
		{
			name:     "亿元 100000000.00 元",
			cents:    10000000000,
			expected: [12]int{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := centsToDigits(tt.cents)
			if result != tt.expected {
				t.Errorf("centsToDigits(%d) = %v, want %v", tt.cents, result, tt.expected)
			}
		})
	}
}

func TestFormatAmountForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		expected string
	}{
		{
			name:     "零值",
			cents:    0,
			expected: "0.00",
		},
		{
			name:     "正整数",
			cents:    123456789,
			expected: "1,234,567.89",
		},
		{
			name:     "负数",
			cents:    -123456789,
			expected: "-1,234,567.89",
		},
		{
			name:     "小金额",
			cents:    1,
			expected: "0.01",
		},
		{
			name:     "整百元",
			cents:    10000,
			expected: "100.00",
		},
		{
			name:     "千元",
			cents:    123400,
			expected: "1,234.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAmountForDisplay(tt.cents)
			if result != tt.expected {
				t.Errorf("formatAmountForDisplay(%d) = %q, want %q", tt.cents, result, tt.expected)
			}
		})
	}
}
