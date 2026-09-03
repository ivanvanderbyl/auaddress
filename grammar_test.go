package anzaddress

import (
	"reflect"
	"testing"
)

func TestRecognizeStreet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected StreetDelivery
	}{
		{
			name:  "slash unit",
			input: "3A/45 High Street",
			expected: StreetDelivery{
				Unit: "3A", StreetNumber: "45", StreetName: "HIGH", StreetType: "ST",
			},
		},
		{
			name:  "unit and level",
			input: "Unit 5 Level 2 100 George Street",
			expected: StreetDelivery{
				Unit: "UNIT 5", Level: "L 2", StreetNumber: "100", StreetName: "GEORGE", StreetType: "ST",
			},
		},
		{
			name:  "compact level",
			input: "L4 54 Wellington Street",
			expected: StreetDelivery{
				Level: "L 4", StreetNumber: "54", StreetName: "WELLINGTON", StreetType: "ST",
			},
		},
		{
			name:  "range and suffix",
			input: "10-12 King George Road North",
			expected: StreetDelivery{
				StreetNumber: "10-12", StreetName: "KING GEORGE", StreetType: "RD", StreetSuffix: "N",
			},
		},
		{
			name:  "split component",
			input: "123 Main\nStreet",
			expected: StreetDelivery{
				StreetNumber: "123", StreetName: "MAIN", StreetType: "ST",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexAddress(tt.input)
			if err != nil {
				t.Fatalf("lexAddress returned an error: %v", err)
			}
			point, next, ok := recognizeStreet(tokens, 0, len(tokens)-1)
			if !ok {
				t.Fatal("expected street recognition to succeed")
			}
			if next != len(tokens)-1 {
				t.Errorf("expected street to consume through %d, got %d", len(tokens)-1, next)
			}
			if !reflect.DeepEqual(point.Street, tt.expected) {
				t.Errorf("street mismatch:\nexpected: %#v\nactual:   %#v", tt.expected, point.Street)
			}
		})
	}
}

func TestRecognizePostal(t *testing.T) {
	tests := []struct {
		input    string
		expected PostalDelivery
	}{
		{"PO Box 42", PostalDelivery{Type: "PO BOX", Number: "42"}},
		{"P.O. Box 42", PostalDelivery{Type: "PO BOX", Number: "42"}},
		{"Locked\nBag 5000", PostalDelivery{Type: "LOCKED BAG", Number: "5000"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := lexAddress(tt.input)
			if err != nil {
				t.Fatalf("lexAddress returned an error: %v", err)
			}
			point, next, ok := recognizePostal(tokens, 0, len(tokens)-1)
			if !ok {
				t.Fatal("expected postal recognition to succeed")
			}
			if next != len(tokens)-1 {
				t.Errorf("expected postal delivery to consume through %d, got %d", len(tokens)-1, next)
			}
			if !reflect.DeepEqual(point.Postal, tt.expected) {
				t.Errorf("postal mismatch: expected %#v, got %#v", tt.expected, point.Postal)
			}
		})
	}
}

func TestRecognizeDeliverySequence(t *testing.T) {
	tests := []struct {
		name  string
		input string
		kinds []DeliveryPointKind
	}{
		{"street then postal", "123 Main Street, PO Box 42", []DeliveryPointKind{DeliveryPointStreet, DeliveryPointPostal}},
		{"postal then street", "PO Box 42\n123 Main Street", []DeliveryPointKind{DeliveryPointPostal, DeliveryPointStreet}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexAddress(tt.input)
			if err != nil {
				t.Fatalf("lexAddress returned an error: %v", err)
			}
			points, next, ok := recognizeDeliverySequence(tokens, 0, len(tokens)-1)
			if !ok {
				t.Fatal("expected delivery sequence recognition to succeed")
			}
			if next != len(tokens)-1 {
				t.Errorf("expected sequence to consume through %d, got %d", len(tokens)-1, next)
			}
			if len(points) != len(tt.kinds) {
				t.Fatalf("expected %d delivery points, got %d", len(tt.kinds), len(points))
			}
			for i, kind := range tt.kinds {
				if points[i].Kind != kind {
					t.Errorf("delivery point %d: expected kind %d, got %d", i, kind, points[i].Kind)
				}
			}
		})
	}
}
