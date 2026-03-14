package domain

import "testing"

func BenchmarkStack_TransitionTo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := &Stack{State: StatePending}
		s.TransitionTo(StateValidating)
		s.TransitionTo(StateInstalling)
		s.TransitionTo(StateConfiguring)
		s.TransitionTo(StateHealthCheck)
		s.TransitionTo(StateCompleted)
	}
}
