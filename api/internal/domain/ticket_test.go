package domain

import "testing"

func TestValidTransition(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		{"claim new ticket", StatusNew, StatusInProgress, true},
		{"cannot wait before claim", StatusNew, StatusWaitingCustomer, false},
		{"wait for customer", StatusInProgress, StatusWaitingCustomer, true},
		{"reopen ticket", StatusResolved, StatusInProgress, true},
		{"cannot reopen directly as new", StatusResolved, StatusNew, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidTransition(tt.from, tt.to); got != tt.want {
				t.Fatalf("ValidTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
