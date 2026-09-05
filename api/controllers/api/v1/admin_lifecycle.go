package v1

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/Improwised/jovvix/api/constants"
	"github.com/Improwised/jovvix/api/models"
	"go.uber.org/zap"
)

// Each Arrange owns cleanup. Its reader alone closes disconnected.
// Arrange alone closes cancelled and done; the question
// loop alone closes quizEnded and questionDone. No sends occur.
type adminLifecycle struct {
	sync.Mutex
	disconnected chan struct{}
	cancelled    chan struct{}
	done         chan struct{}
	quizEnded    chan struct{}
	workers      sync.WaitGroup
}

func newAdminLifecycle() *adminLifecycle {
	return &adminLifecycle{disconnected: make(chan struct{}), cancelled: make(chan struct{}), done: make(chan struct{}), quizEnded: make(chan struct{})}
}

func signalClosed(signal <-chan struct{}) bool {
	select {
	case <-signal:
		return true
	default:
		return false
	}
}

func (l *adminLifecycle) stopped() bool {
	return signalClosed(l.cancelled) || signalClosed(l.done)
}

// Arrange alone invokes this policy after receiving disconnected. A future
// grace period can delay cancellation here without changing timer/worker exits.
func (l *adminLifecycle) cancelAfterAdminDisconnect() {
	close(l.cancelled)
}

func (l *adminLifecycle) cannotWrite() bool {
	return l.stopped() || signalClosed(l.disconnected)
}

func waitAdminCountdown(duration time.Duration, cancelled <-chan struct{}) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-cancelled:
		return false
	case <-timer.C:
		return !signalClosed(cancelled)
	}
}

// Product policy for loss of the host, separate from worker cancellation and
// normal TerminateQuiz. Called only after the disconnect policy cancels workers.
// Ownership is always released after cleanup. A later Arrange must inspect the
// database and reject an already active session before attempting activation.
func (qc *quizSocketController) finishAdminDisconnect(session models.ActiveQuiz) {
	err := qc.activeQuizModel.Deactivate(session.ID)
	if err != nil {
		qc.logger.Error("error deactivating quiz after admin disconnect", zap.Error(err))
	}
	payload, marshalErr := json.Marshal(map[string]any{
		"event":    constants.AdminDisconnected,
		"response": QuizSendResponse{Component: constants.Loading, Data: constants.AdminDisconnected},
	})
	if marshalErr != nil {
		qc.logger.Error("error marshaling admin disconnect", zap.Error(marshalErr))
		return
	}
	if publishErr := qc.redis.PubSubModel.Client.Publish(qc.redis.PubSubModel.Ctx, session.ID.String(), payload).Err(); publishErr != nil {
		qc.logger.Error("error publishing admin disconnect", zap.Error(publishErr))
	}
}

// Clear commands from the previous phase before publishing the next phase.
func drainAdminCommands(commands <-chan bool) {
	for {
		select {
		case <-commands:
		default:
			return
		}
	}
}

// Called once by Arrange, after joining workers. Lobby reports loss through
// isConnected; the in-game reader reports it through disconnected.
func (l *adminLifecycle) finishDisconnectedAdmin(isConnected bool, finish func()) {
	if !isConnected || signalClosed(l.disconnected) {
		finish()
	}
}
