package server

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTheStandSaysWhenThereIsNoPrinter(t *testing.T) {
	w := get(t, testServer(t), "/printer/status")
	var got struct{ Problem string }
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("the page reads this as JSON: %v", err)
	}
	if got.Problem != "No printer selected" {
		t.Errorf("problem = %q", got.Problem)
	}
}

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
