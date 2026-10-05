package server

import (
	"testing"
	"time"
)

func TestAReceiptLeftInTheQueueCountsAsStuck(t *testing.T) {
	s := testStation(t)
	now := time.Now()

	if s.stuck(1, now) {
		t.Errorf("a job just sent is not stuck yet")
	}
	if s.stuck(1, now.Add(stuckAfter/2)) {
		t.Errorf("a job a few seconds old is not stuck yet")
	}
	if !s.stuck(1, now.Add(stuckAfter)) {
		t.Errorf("a job still there after %s is stuck", stuckAfter)
	}
	if s.stuck(0, now.Add(stuckAfter)) {
		t.Errorf("an empty queue is not stuck")
	}
	if s.stuck(1, now.Add(stuckAfter+time.Second)) {
		t.Errorf("the next job starts its own clock")
	}
}

func TestAFailedPrintShowsUntilOneWorks(t *testing.T) {
	s := testStation(t)
	s.st.printFailure("write to printer: device not ready")
	if !s.st.lastPrintFailed() {
		t.Fatalf("a failed print should be remembered")
	}
	s.st.counted()
	if s.st.lastPrintFailed() {
		t.Errorf("a receipt that printed clears it")
	}
}

func TestTheMenuSaysWhenThereIsNoPrinter(t *testing.T) {
	if got := testStation(t).printerProblem(time.Now()); got != "No printer selected" {
		t.Errorf("problem = %q", got)
	}
}
