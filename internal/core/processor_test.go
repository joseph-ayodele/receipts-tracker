package core

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/joseph-ayodele/receipts-tracker/internal/core/llm"
)

func TestAdjustTotalsForTenderOffsets(t *testing.T) {
	tests := []struct {
		name          string
		ocrText       string
		initialFields llm.ReceiptFields
		expectedTotal string
		expectLogged  bool
	}{
		{
			name:    "Amazon gift card scenario - should correct total",
			ocrText: "Item(s) Subtotal: $142.89\nEstimated tax: $12.32\nGift Card Amount: -$155.21\nGrand Total: $0.00",
			initialFields: llm.ReceiptFields{
				Total: "0.00",
			},
			expectedTotal: "155.21",
			expectLogged:  true,
		},
		{
			name:    "Store credit scenario - should correct total",
			ocrText: "Subtotal: $89.50\nTax: $7.16\nStore Credit: -$96.66\nTotal: $0.00",
			initialFields: llm.ReceiptFields{
				Total: "0.00",
			},
			expectedTotal: "96.66",
			expectLogged:  true,
		},
		{
			name:    "Promo balance scenario - should correct total",
			ocrText: "Item(s) Subtotal: $234.56\nEstimated tax: $18.76\nPromo Balance: -$253.32\nGrand Total: $0.00",
			initialFields: llm.ReceiptFields{
				Total: "0.00",
			},
			expectedTotal: "253.32",
			expectLogged:  true,
		},
		{
			name:    "With discount - should apply discount then compute total",
			ocrText: "Item(s) Subtotal: $100.00\nDiscount: -$10.00\nTax: $7.20\nGift Card Amount: -$97.20\nGrand Total: $0.00",
			initialFields: llm.ReceiptFields{
				Total: "0.00",
			},
			expectedTotal: "97.20",
			expectLogged:  true,
		},
		{
			name:    "With fees and tip - should include all components",
			ocrText: "Item(s) Subtotal: $75.00\nOther fees: $5.00\nTip: $15.00\nTax: $7.35\nStore Credit: -$102.35\nGrand Total: $0.00",
			initialFields: llm.ReceiptFields{
				Total: "0.00",
			},
			expectedTotal: "102.35",
			expectLogged:  true,
		},
		{
			name:    "Non-zero total - should not adjust",
			ocrText: "Subtotal: $50.00\nTax: $4.00\nTotal: $54.00",
			initialFields: llm.ReceiptFields{
				Total: "54.00",
			},
			expectedTotal: "54.00",
			expectLogged:  false,
		},
		{
			name:    "No tender keywords - should not adjust",
			ocrText: "Subtotal: $50.00\nTax: $4.00\nCoupon: -$5.00\nTotal: $0.00",
			initialFields: llm.ReceiptFields{
				Total: "0.00",
			},
			expectedTotal: "0.00",
			expectLogged:  false,
		},
		{
			name:    "Empty total field - should not adjust",
			ocrText: "Item(s) Subtotal: $142.89\nEstimated tax: $12.32\nGift Card Amount: -$155.21\nGrand Total: $0.00",
			initialFields: llm.ReceiptFields{
				Total: "",
			},
			expectedTotal: "",
			expectLogged:  false,
		},
		{
			name:    "Nil fields - should not panic",
			ocrText: "Item(s) Subtotal: $142.89\nEstimated tax: $12.32\nGift Card Amount: -$155.21\nGrand Total: $0.00",
			initialFields: llm.ReceiptFields{
				Total: "0.00",
			},
			expectedTotal: "155.21",
			expectLogged:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a logger that captures log output for testing
			var logOutput strings.Builder
			logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}))

			fields := tt.initialFields
			adjustTotalsForTenderOffsets(tt.ocrText, &fields, logger)

			if fields.Total != tt.expectedTotal {
				t.Errorf("Expected total %s, got %s", tt.expectedTotal, fields.Total)
			}

			logged := logOutput.Len() > 0
			if logged != tt.expectLogged {
				t.Errorf("Expected logged=%v, got logged=%v", tt.expectLogged, logged)
				if logged {
					t.Logf("Log output: %s", logOutput.String())
				}
			}
		})
	}
}

func TestParseMoney(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		valid    bool
	}{
		{"$123.45", 123.45, true},
		{"$0.00", 0.00, true},
		{"-$10.50", -10.50, true},
		{"123.45", 123.45, true},
		{"$123", 123.00, true},
		{"$123.456", 0, false}, // too many decimal places
		{"abc", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			val, ok := parseMoney(tt.input)
			if ok != tt.valid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.valid, ok)
			}
			if ok && val != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, val)
			}
		})
	}
}

func TestFindMoneyAfter(t *testing.T) {
	tests := []struct {
		text     string
		labels   []string
		expected float64
		found    bool
	}{
		{
			"Subtotal: $142.89\nEstimated tax: $12.32",
			[]string{"subtotal"},
			142.89,
			true,
		},
		{
			"Subtotal: $89.50\nTax: $7.16",
			[]string{"subtotal"},
			89.50,
			true,
		},
		{
			"Discount: -$10.00\nTotal: $50.00",
			[]string{"discount"},
			-10.00,
			true,
		},
		{
			"No money here",
			[]string{"subtotal"},
			0,
			false,
		},
		{
			"Subtotal: $100.00\nSubtotal: $200.00",
			[]string{"subtotal"},
			100.00,
			true,
		},
	}

	for _, tt := range tests {
		textLen := len(tt.text)
		if textLen > 20 {
			textLen = 20
		}
		t.Run(fmt.Sprintf("%s_%v", tt.text[:textLen], tt.labels), func(t *testing.T) {
			val, found := findMoneyAfter(tt.text, tt.labels...)
			if found != tt.found {
				t.Errorf("Expected found=%v, got found=%v", tt.found, found)
			}
			if found && val != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, val)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		text     string
		needles  []string
		expected bool
	}{
		{"gift card amount", []string{"gift card"}, true},
		{"gift card amount", []string{"gift card"}, true},
		{"promo balance", []string{"promo balance"}, true},
		{"store credit", []string{"store credit"}, true},
		{"no match here", []string{"gift card"}, false},
		{"", []string{"gift card"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := containsAny(tt.text, tt.needles...)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
