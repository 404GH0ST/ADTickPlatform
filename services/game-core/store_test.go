package main

import (
	"math"
	"testing"
)

func TestFaustDefensePenalty(t *testing.T) {
	tests := []struct {
		name         string
		captureCount int
		want         float64
	}{
		{name: "uncaptured flag has no penalty", captureCount: 0, want: 0},
		{name: "single capture subtracts one point", captureCount: 1, want: 1},
		{name: "multiple captures use exponent", captureCount: 2, want: math.Pow(2, 0.75)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := faustDefensePenalty(tt.captureCount); math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("faustDefensePenalty(%d) = %f, want %f", tt.captureCount, got, tt.want)
			}
		})
	}
}

func TestFaustAttackValueUsesCaptureCount(t *testing.T) {
	tests := []struct {
		name         string
		captureCount int
		want         float64
	}{
		{name: "single capture gets full bonus", captureCount: 1, want: 2.0},
		{name: "second attacker still gets one and a half", captureCount: 2, want: 1.5},
		{name: "third attacker gets one and a third", captureCount: 3, want: 1.0 + (1.0 / 3.0)},
		{name: "invalid capture count stays zero", captureCount: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := faustAttackValue(tt.captureCount); math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("faustAttackValue(%d) = %f, want %f", tt.captureCount, got, tt.want)
			}
		})
	}
}

func TestFaustSLAValueMapping(t *testing.T) {
	tests := []struct {
		name    string
		putOK   bool
		getOK   bool
		checkOK bool
		want    float64
	}{
		{name: "all phases successful is ok", putOK: true, getOK: true, checkOK: true, want: 1.0},
		{name: "get and check only is recovering", putOK: false, getOK: true, checkOK: true, want: 0.5},
		{name: "missing get is down", putOK: true, getOK: false, checkOK: true, want: 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := faustSLAValue(tt.putOK, tt.getOK, tt.checkOK); math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("faustSLAValue(%t, %t, %t) = %f, want %f", tt.putOK, tt.getOK, tt.checkOK, got, tt.want)
			}
		})
	}
}
