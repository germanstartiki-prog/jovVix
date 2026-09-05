package v1

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Improwised/jovvix/api/constants"
	"github.com/Improwised/jovvix/api/models"
	pubsub "github.com/Improwised/jovvix/api/pkg/redis"
	"github.com/doug-martin/goqu/v9"
	ws "github.com/fasthttp/websocket"
	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func lobbyTestSocket(t *testing.T, handler func(*websocket.Conn)) (*ws.Conn, <-chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		c, err := (&ws.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()
		handler(&websocket.Conn{Conn: c})
	}))
	t.Cleanup(server.Close)
	client, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client, done
}

func waitLobbyTest(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lobby handler did not finish")
	}
}

func TestHandleStartQuizClose(t *testing.T) {
	for _, code := range []int{ws.CloseNormalClosure, ws.CloseGoingAway} {
		core, logs := observer.New(zap.DebugLevel)
		client, done := lobbyTestSocket(t, func(c *websocket.Conn) {
			connected := true
			event := handleStartQuiz(c, zap.New(core), &connected, constants.ActionAuthentication, newAdminLifecycle())
			if connected || event != constants.AdminDisconnected {
				t.Errorf("close %d: connected=%v event=%s", code, connected, event)
			}
		})
		if err := client.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(code, "leaving"), time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		waitLobbyTest(t, done)
		if logs.FilterLevelExact(zap.ErrorLevel).Len() != 0 || logs.FilterLevelExact(zap.InfoLevel).Len() != 1 {
			t.Fatal("expected one INFO and no ERROR for close", code)
		}
	}
}

// Only the roster watcher may GET. A closed client prevents real Redis I/O;
// this hook supplies its empty roster without involving any database.
type lobbyRosterHook struct{ gets atomic.Int32 }

func (h *lobbyRosterHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (h *lobbyRosterHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *lobbyRosterHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "get" {
			h.gets.Add(1)
			cmd.(*redis.StringCmd).SetVal("[]")
			return nil
		}
		return next(ctx, cmd)
	}
}

func TestLobbyDoesNotSendFailureAfterDisconnect(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	rc := redis.NewClient(&redis.Options{})
	hook := &lobbyRosterHook{}
	rc.AddHook(hook)
	_ = rc.Close()
	qc := &quizSocketController{logger: zap.New(core), redis: &pubsub.RedisPubSub{PubSubModel: &pubsub.PubSubModel{Ctx: context.Background(), Client: *rc}}}
	client, done := lobbyTestSocket(t, func(c *websocket.Conn) {
		l := newAdminLifecycle()
		connected := true
		handleCodeGeneration(c, qc, models.ActiveQuiz{}, &connected, &QuizSendResponse{}, l.done, l)
		close(l.done)
		l.workers.Wait()
		if connected {
			t.Error("lobby retained connected status")
		}
	})
	if err := client.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseGoingAway, "leaving"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	waitLobbyTest(t, done)
	if hook.gets.Load() != 1 {
		t.Fatalf("unexpected GET after disconnect: %d GETs", hook.gets.Load())
	}
	if logs.FilterMessageSnippet("socket error middleware").Len() != 0 {
		t.Fatal("JSONFailWs attempted after disconnect")
	}
}

type failedSessionActivator struct{ err error }

func (m failedSessionActivator) GetOrActivateSession(string, string) (models.ActiveQuiz, error) {
	return models.ActiveQuiz{}, m.err
}

func TestActivateAndGetSessionPreservesModelError(t *testing.T) {
	for _, message := range []string{constants.ErrSessionWasCompleted, constants.Unauthenticated, "database unavailable"} {
		modelErr := errors.New(message)
		client, done := lobbyTestSocket(t, func(c *websocket.Conn) {
			_, err := ActivateAndGetSession(c, failedSessionActivator{modelErr}, zap.NewNop(), "session", "admin", newAdminLifecycle())
			if err != modelErr {
				t.Errorf("model error replaced: got %v want %v", err, modelErr)
			}
		})
		client.SetReadDeadline(time.Now().Add(time.Second))
		if _, _, err := client.ReadMessage(); err != nil {
			t.Fatalf("expected error response: %v", err)
		}
		waitLobbyTest(t, done)
	}
}

func TestHandleStartQuizUnexpectedReadError(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	client, done := lobbyTestSocket(t, func(c *websocket.Conn) {
		connected := true
		event := handleStartQuiz(c, zap.New(core), &connected, "start", newAdminLifecycle())
		if connected || event != constants.UnknownError {
			t.Error("invalid JSON was treated as expected close")
		}
	})
	if err := client.WriteMessage(ws.TextMessage, []byte("{")); err != nil {
		t.Fatal(err)
	}
	waitLobbyTest(t, done)
	if logs.FilterLevelExact(zap.ErrorLevel).Len() != 1 {
		t.Fatal("unexpected error was suppressed")
	}
}

func TestCancelledLobbyDoesNotReadOrWrite(t *testing.T) {
	for _, signal := range []string{"disconnected", "cancelled", "done"} {
		l := newAdminLifecycle()
		switch signal {
		case "disconnected":
			close(l.disconnected)
		case "cancelled":
			close(l.cancelled)
		case "done":
			close(l.done)
		}
		connected := true
		// Nil dependencies ensure the cancelled path performs neither I/O nor writes.
		handleCodeGeneration(nil, nil, models.ActiveQuiz{}, &connected, nil, l.done, l)
		if connected {
			t.Errorf("%s: connection still marked alive", signal)
		}
	}
}

func TestLobbyCloseRunsEmergencyCleanupOnce(t *testing.T) {
	dbState := &cleanupDB{t: t}
	db := sql.OpenDB(dbState)
	defer db.Close()
	rc := redis.NewClient(&redis.Options{})
	defer rc.Close()
	hook := &cleanupPublishHook{t: t}
	rc.AddHook(hook)
	qc := &quizSocketController{
		logger:          zap.NewNop(),
		activeQuizModel: models.InitActiveQuizModel(goqu.New("postgres", db), zap.NewNop()),
		redis:           &pubsub.RedisPubSub{PubSubModel: &pubsub.PubSubModel{Ctx: context.Background(), Client: *rc}},
	}
	session := models.ActiveQuiz{ID: uuid.New()}
	client, done := lobbyTestSocket(t, func(c *websocket.Conn) {
		l := newAdminLifecycle()
		connected := true
		handleStartQuiz(c, zap.NewNop(), &connected, constants.ActionAuthentication, l)
		l.workers.Wait()
		l.finishDisconnectedAdmin(connected, func() { qc.finishAdminDisconnect(session) })
	})
	if err := client.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseNormalClosure, "leaving"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	waitLobbyTest(t, done)
	if dbState.calls != 1 || hook.calls != 1 {
		t.Fatalf("Deactivate calls=%d, AdminDisconnected publications=%d", dbState.calls, hook.calls)
	}
}

func TestNormalCompletionSkipsEmergencyCleanup(t *testing.T) {
	l := newAdminLifecycle()
	close(l.quizEnded)
	close(l.done)
	l.finishDisconnectedAdmin(true, func() { t.Fatal("normal completion invoked emergency cleanup") })
}

// Exercise the actual model update and Redis publication without external services.
type cleanupDB struct {
	t     *testing.T
	calls int
}

func (d *cleanupDB) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *cleanupDB) Driver() driver.Driver                        { return d }
func (d *cleanupDB) Open(string) (driver.Conn, error)             { return d, nil }
func (d *cleanupDB) Close() error                                 { return nil }
func (d *cleanupDB) Begin() (driver.Tx, error)                    { return nil, errors.New("unexpected transaction") }
func (d *cleanupDB) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *cleanupDB) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	d.calls++
	if !strings.Contains(query, `"is_active"=false`) && !strings.Contains(query, `"is_active"=FALSE`) {
		d.t.Errorf("expected session deactivation: %s", query)
	}
	return driver.RowsAffected(1), nil
}

type cleanupPublishHook struct {
	lobbyRosterHook
	t     *testing.T
	calls int
}

func (h *cleanupPublishHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() != "publish" {
			h.t.Errorf("unexpected command: %s", cmd.Name())
			return nil
		}
		h.calls++
		payload, ok := cmd.Args()[2].([]byte)
		if !ok || !strings.Contains(string(payload), constants.AdminDisconnected) {
			h.t.Error("missing AdminDisconnected event")
		}
		cmd.(*redis.IntCmd).SetVal(1)
		return nil
	}
}
