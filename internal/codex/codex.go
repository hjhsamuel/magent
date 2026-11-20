package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrRuntimeConnect distinguishes connection failures from initialization failures.
var ErrRuntimeConnect = errors.New("connect codex runtime")

const requestTimeout = 5 * time.Second

// Runtime owns one WebSocket connection to the remote app-server.
// Protocol: https://learn.chatgpt.com/docs/app-server
// Consume Events continuously and use Approve for command/file approval requests.
// When Events is full, text deltas are dropped and other events apply backpressure.
// Keep draining Events independently of RPC/approval calls: backpressure pauses
// the shared protocol reader, including responses. Final items must reconcile deltas.
type Runtime struct {
	conn               MessageTransport
	threadID           string
	exited             chan struct{}
	mu                 sync.Mutex
	writeMu            sync.Mutex
	pending            map[string]chan Message
	approvals          map[string]*approvalWaiter
	requestsByPublicID map[string]cachedRequest
	serverIDs          map[string]string
	nextID             uint64
	events             chan Message
	done               chan struct{}
	err                error
	once               sync.Once
}

type cachedRequest struct {
	message  Message
	serverID string // JSON-encoded ID used on the wire.
}

type approvalWaiter struct {
	publicID string
	done     chan struct{}
}

// ApprovalRequests returns a snapshot of cached requests keyed by the JSON-encoded
// public ID from Events. Each message carries that same ID for replies.
func (r *Runtime) ApprovalRequests() map[string]Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := make(map[string]Message, len(r.requestsByPublicID))
	for publicID, request := range r.requestsByPublicID {
		requests[publicID] = cloneRequest(request.message)
	}
	return requests
}

func cloneRequest(m Message) Message {
	m.ID = bytes.Clone(m.ID)
	m.Params = bytes.Clone(m.Params)
	m.Result = bytes.Clone(m.Result)
	if m.Error != nil {
		e := *m.Error
		e.Data = bytes.Clone(e.Data)
		m.Error = &e
	}
	return m
}

func (r *Runtime) init(ctx context.Context) error {
	if err := r.call(
		ctx,
		MethodInitialize,
		InitializeParams{
			Capabilities: &InitializeCapabilities{OptOutNotificationMethods: ignoredNotificationMethods()},
			ClientInfo: ClientInfo{
				Name:    "magent",
				Version: "0.1.0",
			},
		},
		&InitializeResponse{},
	); err != nil {
		return err
	}

	data, err := json.Marshal(InitializedParams{})
	if err != nil {
		return err
	}
	if err := r.write(&Message{
		Method: MethodInitialized,
		Params: data,
	}); err != nil {
		return err
	}
	return nil
}

// Events delivers content, progress, lifecycle and error notifications plus server requests.
// Auxiliary notifications listed in ignoredNotificationMethods are filtered out.
func (r *Runtime) Events() <-chan Message {
	return r.events
}

func (r *Runtime) Done() <-chan struct{} {
	return r.done
}

func (r *Runtime) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *Runtime) Close() error {
	r.stop(errors.New("codex runtime closed"))
	r.WaitClosed()
	return nil
}

func (r *Runtime) stop(err error) {
	r.once.Do(func() {
		r.mu.Lock()
		r.err = err
		r.requestsByPublicID = nil
		r.serverIDs = nil
		close(r.done)
		r.mu.Unlock()
		_ = r.conn.Close()
	})
}

func (r *Runtime) write(m *Message) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	select {
	case <-r.done:
		return r.Err()
	default:
	}
	if err := r.conn.WriteMessage(m); err != nil {
		r.stop(err)
		return err
	}
	return nil
}

func (r *Runtime) readLoop() {
	if r.exited != nil {
		defer close(r.exited)
	}
	defer close(r.events)

	for {
		var m Message
		if err := r.conn.ReadMessage(&m); err != nil {
			r.stop(err)
			return
		}

		if m.Method == "" && len(m.ID) > 0 {
			r.mu.Lock()
			ch := r.pending[string(m.ID)]
			r.mu.Unlock()

			if ch != nil {
				select {
				case ch <- m:
				default:
				}
			}
			continue
		}

		if m.Method == MethodServerRequestResolved && len(m.ID) == 0 {
			var resolved ServerRequestResolvedParams
			if err := json.Unmarshal(m.Params, &resolved); err != nil {
				r.stop(fmt.Errorf("invalid serverRequest/resolved: %w", err))
				return
			}

			r.mu.Lock()
			key := string(bytes.TrimSpace(resolved.RequestID))
			publicID, known := r.serverIDs[key]
			delete(r.serverIDs, key)
			delete(r.requestsByPublicID, publicID)
			if waiter := r.approvals[key]; waiter != nil {
				publicID, known = waiter.publicID, true
				delete(r.approvals, key)
				close(waiter.done)
			}
			r.mu.Unlock()

			if !known {
				continue
			}

			resolved.RequestID = json.RawMessage(publicID)
			m.Params, _ = json.Marshal(resolved)
		}

		if len(m.ID) > 0 {
			public, err := newRequestID()
			if err != nil {
				r.stop(fmt.Errorf("generate request ID: %w", err))
				return
			}
			publicKey := string(public)
			key := string(bytes.TrimSpace(m.ID))

			r.mu.Lock()

			select {
			case <-r.done:
				r.mu.Unlock()
				return
			default:
			}

			if r.requestsByPublicID == nil {
				r.requestsByPublicID = make(map[string]cachedRequest)
				r.serverIDs = make(map[string]string)
			}
			if _, exists := r.serverIDs[key]; exists || r.approvals[key] != nil {
				r.mu.Unlock()
				r.stop(errors.New("duplicate server request ID"))
				return
			}
			m.ID = public
			r.requestsByPublicID[publicKey] = cachedRequest{
				message:  cloneRequest(m),
				serverID: key,
			}
			r.serverIDs[key] = publicKey
			r.mu.Unlock()
		}

		if ignoreNotification(m) {
			continue
		}

		if droppableEvent(m) {
			select {
			case <-r.done:
				return
			case r.events <- m:
			default:
			}
			continue
		}
		select {
		case <-r.done:
			return
		case r.events <- m:
		}
	}
}

// call sends a request. Cancellation stops waiting, not the remote operation.
func (r *Runtime) call(ctx context.Context, method Method, params, result any) (callErr error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	defer func() {
		if callErr != nil {
			callErr = fmt.Errorf("codex %s: %w", method, callErr)
		}
	}()

	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.nextID++
	id := fmt.Sprintf(`"codex_%d"`, r.nextID)
	ch := make(chan Message, 1)
	r.pending[id] = ch
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.pending, id)
		r.mu.Unlock()
	}()

	// A blocked transport write must also be interruptible. Canceling it closes
	// this connection, since a partially written frame cannot safely be reused.
	sent := make(chan error, 1)
	go func() {
		sent <- r.write(&Message{
			ID:     json.RawMessage(id),
			Method: method,
			Params: data,
		})
	}()

	select {
	case err = <-sent:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		r.stop(ctx.Err())
		return ctx.Err()
	case <-r.done:
		return r.Err()
	}

	select {
	case m := <-ch:
		if (m.Error != nil && len(m.Result) != 0) ||
			(m.Error == nil && (len(m.Result) == 0 || bytes.Equal(bytes.TrimSpace(m.Result), []byte("null")))) {
			return errors.New("invalid response: expected exactly one non-null result or error")
		}
		if m.Error != nil {
			return m.Error
		}
		if result != nil {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-r.done:
		return r.Err()
	}
}

// reply answers a server request using its original numeric or string ID.
// Pass an RPCError to reject an unsupported request or an approval.
func (r *Runtime) reply(id json.RawMessage, result any, rpcErr *RPCError) error {
	var value any
	if json.Unmarshal(id, &value) != nil {
		return errors.New("invalid request ID")
	}
	switch value.(type) {
	case string, float64:
	default:
		return errors.New("request ID must be a string or number")
	}
	m := &Message{ID: id, Error: rpcErr}
	if rpcErr == nil {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		m.Result = data
	}
	return r.write(m)
}

// ThreadStart creates a thread and binds the Runtime after validating the response.
func (r *Runtime) ThreadStart(ctx context.Context, params ThreadStartParams) (ThreadResponse, error) {
	if r.threadID != "" {
		return ThreadResponse{}, errors.New("runtime already bound to a thread")
	}
	var result ThreadResponse
	err := r.call(ctx, MethodThreadStart, params, &result)
	if err == nil && result.Thread.ID == "" {
		err = errors.New("codex thread/start: response missing thread ID")
	}
	if err == nil {
		r.threadID = result.Thread.ID
	}
	return result, err
}

func (r *Runtime) ThreadResume(ctx context.Context, params ThreadResumeParams) (ThreadResponse, error) {
	if r.threadID != "" && r.threadID != params.ThreadID {
		return ThreadResponse{}, errors.New("runtime thread mismatch")
	}
	var result ThreadResponse
	err := r.call(ctx, MethodThreadResume, params, &result)
	if err == nil && (result.Thread.ID == "" || result.Thread.ID != params.ThreadID) {
		err = errors.New("codex thread/resume: response thread ID mismatch")
	}
	return result, err
}

// TurnStart returns the acknowledgement; completion arrives via Events.
func (r *Runtime) TurnStart(ctx context.Context, params TurnStartParams) (TurnStartResponse, error) {
	if r.threadID != "" && r.threadID != params.ThreadID {
		return TurnStartResponse{}, errors.New("runtime thread mismatch")
	}
	var result TurnStartResponse
	err := r.call(ctx, MethodTurnStart, params, &result)
	if err == nil && result.Turn.ID == "" {
		err = errors.New("codex turn/start: response missing turn ID")
	}
	return result, err
}

func (r *Runtime) TurnInterrupt(ctx context.Context, params TurnInterruptParams) error {
	if r.threadID != "" && r.threadID != params.ThreadID {
		return errors.New("runtime thread mismatch")
	}
	return r.call(ctx, MethodTurnInterrupt, params, &TurnInterruptResponse{})
}

// Approve replies to a command-execution or file-change approval request.
// id is the opaque service-generated request ID from Events.
// This operation supports command-execution and file-change decisions only.
func (r *Runtime) Approve(
	ctx context.Context,
	id json.RawMessage,
	decision ApprovalDecision,
) error {
	return r.replyAndWait(ctx, id, ApprovalResponse{Decision: decision})
}

// ApprovePermissions grants only the subset explicitly supplied by the caller.
// Decline and Cancel ignore supplied permissions and grant nothing.
func (r *Runtime) ApprovePermissions(
	ctx context.Context,
	id json.RawMessage,
	decision ApprovalDecision,
	permission GrantedPermissionProfile,
) error {
	response := PermissionsRequestApprovalResponse{
		Scope: PermissionScopeTurn,
	}
	switch decision {
	case ApprovalAccept, ApprovalAcceptForSession:
		response.Permissions = permission
		if decision == ApprovalAcceptForSession {
			response.Scope = PermissionScopeSession
		}
	case ApprovalDecline, ApprovalCancel:

	default:
		return fmt.Errorf("unsupported approval decision: %q", decision)
	}
	return r.replyAndWait(ctx, id, response)
}

// RespondElicitation answers an MCP form or URL request received through Events.
// Decline and Cancel ignore content and send null.
func (r *Runtime) RespondElicitation(
	ctx context.Context,
	id json.RawMessage,
	decision ApprovalDecision,
	content json.RawMessage,
) error {
	response := McpServerElicitationRequestResponse{}
	switch decision {
	case ApprovalAccept:
		response.Action = McpElicitationAccept
		response.Content = content
	case ApprovalDecline:
		response.Action = McpElicitationDecline
	case ApprovalCancel:
		response.Action = McpElicitationCancel
	default:
		return fmt.Errorf("unsupported elicitation decision: %q", decision)
	}
	return r.replyAndWait(ctx, id, response)
}

// Resolution confirms the server cleared the request, not that work was granted.
func (r *Runtime) replyAndWait(ctx context.Context, id json.RawMessage, response any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	publicID := string(bytes.TrimSpace(id))
	ack := make(chan struct{})

	r.mu.Lock()
	request, exists := r.requestsByPublicID[publicID]
	if !exists {
		r.mu.Unlock()
		return errors.New("unknown or expired server request ID")
	}
	key := request.serverID
	if r.approvals == nil {
		r.approvals = make(map[string]*approvalWaiter)
	}
	if _, exists := r.approvals[key]; exists {
		r.mu.Unlock()
		return errors.New("server request response already pending")
	}
	waiter := &approvalWaiter{publicID: publicID, done: ack}
	r.approvals[key] = waiter
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		if r.approvals[key] == waiter {
			delete(r.approvals, key)
		}
		r.mu.Unlock()
	}()

	sent := make(chan error, 1)
	go func() {
		sent <- r.reply(json.RawMessage(key), response, nil)
	}()

	select {
	case err := <-sent:
		if err != nil {
			return fmt.Errorf("send server request response: %w", err)
		}
		r.mu.Lock()
		delete(r.requestsByPublicID, publicID)
		if r.serverIDs[key] == publicID {
			delete(r.serverIDs, key)
		}
		r.mu.Unlock()
	case <-ctx.Done():
		r.stop(ctx.Err())
		return fmt.Errorf("send server request response: %w", ctx.Err())
	case <-r.done:
		return r.Err()
	}

	select {
	case <-ack:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("server request resolution not confirmed: %w", ctx.Err())
	case <-r.done:
		return fmt.Errorf("server request resolution not confirmed: %w", r.Err())
	}
}

// WaitClosed waits for the reader to exit without initiating shutdown.
func (r *Runtime) WaitClosed() {
	if r.exited != nil {
		<-r.exited
	}
}

// ThreadID identifies a Runtime opened and bound by UserRuntime.
func (r *Runtime) ThreadID() string {
	return r.threadID
}

// OpenThread creates or resumes a thread and binds it before publishing the Runtime.
// Call this before sharing the Runtime with other goroutines.
func (r *Runtime) OpenThread(ctx context.Context, threadID string) (string, error) {
	if r.threadID != "" {
		return "", errors.New("runtime already bound to a thread")
	}

	var (
		response ThreadResponse
		err      error
	)
	if threadID == "" {
		response, err = r.ThreadStart(ctx, ThreadStartParams{})
	} else {
		response, err = r.ThreadResume(ctx, ThreadResumeParams{
			ThreadID:     threadID,
			ExcludeTurns: true,
		})
	}
	if err != nil {
		return "", err
	}

	r.threadID = response.Thread.ID
	return r.threadID, nil
}

// ThreadRead queries a thread without resuming or binding it.
func (r *Runtime) ThreadRead(ctx context.Context, params ThreadReadParams) (ThreadResponse, error) {
	var response ThreadResponse
	if err := r.call(ctx, MethodThreadRead, params, &response); err != nil {
		return response, err
	}
	if response.Thread.ID != params.ThreadID || response.Thread.Status.Type == "" {
		return response, fmt.Errorf("invalid thread/read response for %q", params.ThreadID)
	}
	return response, nil
}

func NewRuntimeContext(ctx context.Context, endpoint string, headers http.Header) (*Runtime, error) {
	c, err := NewWebSocketConnContext(ctx, endpoint, headers)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRuntimeConnect, err)
	}

	r := newRuntime(c)
	if err := r.init(ctx); err != nil {
		r.Close()
		return nil, fmt.Errorf("initialize codex runtime: %w", err)
	}
	return r, nil
}

func newRuntime(c MessageTransport) *Runtime {
	r := &Runtime{
		conn:    c,
		pending: make(map[string]chan Message),
		events:  make(chan Message, 1024),
		done:    make(chan struct{}),
		exited:  make(chan struct{}),
	}
	go r.readLoop()
	return r
}

// newRequestID generates a UUID v4 as a JSON string outside the Runtime lock.
func newRequestID() (json.RawMessage, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	return strconv.AppendQuote(make(json.RawMessage, 0, 38), id.String()), nil
}
