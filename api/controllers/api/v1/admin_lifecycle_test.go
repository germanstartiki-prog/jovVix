package v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Improwised/jovvix/api/constants"
	"github.com/Improwised/jovvix/api/models"
	ws "github.com/fasthttp/websocket"
	"github.com/gofiber/contrib/websocket"
	"go.uber.org/zap"
)

func TestAdminReaderDisconnect(t *testing.T) {
	for _, normal := range []bool{false, true} {
		name := "disconnect"
		if normal {
			name = "normal completion"
		}
		t.Run(name, func(t *testing.T) {
			l := newAdminLifecycle()
			ended := make(chan struct{})
			qc := &quizSocketController{logger: zap.NewNop()}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, err := (&ws.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					close(ended)
					return
				}
				defer c.Close()
				defer close(ended)
				listenAllEvents(&websocket.Conn{Conn: c}, qc, make(chan bool), make(chan bool), make(chan bool), make(chan bool), l.quizEnded, l)
			}))
			defer server.Close()
			client, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			// An out-of-phase command with no receiver must not prevent reading Close.
			if err := client.WriteJSON(QuizReceiveResponse{Event: constants.EventSkipTimer}); err != nil {
				t.Fatal(err)
			}
			if normal {
				close(l.quizEnded)
			}
			if err := client.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseGoingAway, "leaving"), time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			select {
			case <-ended:
			case <-time.After(2 * time.Second):
				t.Fatal("reader did not finish")
			}
			if signalClosed(l.disconnected) == normal {
				t.Fatal("incorrect disconnect classification")
			}
		})
	}
}

func TestAdminCountdownCancellation(t *testing.T) {
	l := newAdminLifecycle()
	result := make(chan bool, 1)
	go func() { result <- waitAdminCountdown(time.Hour, l.cancelled) }()
	close(l.cancelled)
	select {
	case completed := <-result:
		if completed {
			t.Fatal("cancelled countdown completed")
		}
	case <-time.After(time.Second):
		t.Fatal("countdown stuck")
	}
}

func TestAdminScoreTimerCancellation(t *testing.T) {
	l := newAdminLifecycle()
	var wg sync.WaitGroup
	wg.Add(1)
	go handleSkipTimer(nil, nil, &wg, &QuizSendResponse{}, models.ActiveQuiz{}, make(chan bool), make(chan bool), 3600, l)
	close(l.cancelled)
	ended := make(chan struct{})
	go func() { wg.Wait(); close(ended) }()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("score timer stuck")
	}
}

func TestPausedAdminScoreTimerCancellation(t *testing.T) {
	l := newAdminLifecycle()
	pause := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(1)
	go handleSkipTimer(nil, nil, &wg, &QuizSendResponse{}, models.ActiveQuiz{}, make(chan bool), pause, 3600, l)
	// An unpause command establishes that the timer goroutine is running.
	select {
	case pause <- false:
	case <-time.After(time.Second):
		t.Fatal("timer did not start")
	}
	// Suppress Redis publication in this isolated timer test.
	close(l.done)
	select {
	case pause <- true:
	case <-time.After(time.Second):
		t.Fatal("pause blocked")
	}
	close(l.cancelled)
	ended := make(chan struct{})
	go func() { wg.Wait(); close(ended) }()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("paused timer stuck")
	}
}

func TestDisconnectReasonDoesNotCancelWorkersUntilPolicy(t *testing.T) {
	l := newAdminLifecycle()
	close(l.disconnected)
	if l.stopped() {
		t.Fatal("reader bypassed disconnect policy")
	}
	if !l.cannotWrite() {
		t.Fatal("writes allowed after disconnect")
	}
	l.cancelAfterAdminDisconnect()
	if !l.stopped() {
		t.Fatal("policy did not cancel workers")
	}
}
