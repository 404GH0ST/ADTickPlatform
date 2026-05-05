package main

import "testing"

func TestBoundedDefenseScore(t *testing.T) {
	tests := []struct {
		name   string
		issued int
		stolen int
		want   int
	}{
		{name: "no issued flags starts at max", issued: 0, stolen: 0, want: 1000},
		{name: "no stolen flags stays at max", issued: 8, stolen: 0, want: 1000},
		{name: "partial stolen flags reduce proportionally", issued: 10, stolen: 3, want: 730},
		{name: "all stolen flags hit floor", issued: 10, stolen: 10, want: 100},
		{name: "overcounted stolen flags stay floored", issued: 10, stolen: 12, want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := boundedDefenseScore(tt.issued, tt.stolen); got != tt.want {
				t.Fatalf("boundedDefenseScore(%d, %d) = %d, want %d", tt.issued, tt.stolen, got, tt.want)
			}
		})
	}
}

func TestAttackSubmissionValueDiminishesByCaptureIndex(t *testing.T) {
	tests := []struct {
		name         string
		weight       int
		captureIndex int
		want         int
	}{
		{name: "first capture gets full value", weight: 1, captureIndex: 1, want: 10},
		{name: "second capture gets half value", weight: 1, captureIndex: 2, want: 5},
		{name: "third capture rounds inverse value", weight: 1, captureIndex: 3, want: 3},
		{name: "higher challenge weight scales base", weight: 3, captureIndex: 2, want: 15},
		{name: "very late capture keeps minimum value", weight: 1, captureIndex: 99, want: 1},
		{name: "zero weight stays zero", weight: 0, captureIndex: 1, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attackSubmissionValue(tt.weight, tt.captureIndex); got != tt.want {
				t.Fatalf("attackSubmissionValue(%d, %d) = %d, want %d", tt.weight, tt.captureIndex, got, tt.want)
			}
		})
	}
}
