// Package client is the typed gRPC client the CLI uses to talk to mgt-be.
//
// The exported types (Me, Repo, Stack, …) are deliberately the same shape
// as before the gRPC migration, so callers in cmd/ don't need to know the
// transport changed. Internally each method dials a *grpc.ClientConn (lazy)
// and translates between proto messages and the Go structs below.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	mgtv1 "github.com/santhosh/mgt-proto/gen/mgt/v1"

	"github.com/santhosh/mgt/pkg/config"

	"crypto/tls"
)

// Client is the public-surface gRPC client used by CLI commands.
type Client struct {
	Addr     string
	Token    string
	Insecure bool

	mu   sync.Mutex
	conn *grpc.ClientConn
	rpc  mgtv1.MgtServiceClient
}

// New returns a client configured from ~/.mgt and env. The connection is
// not opened until the first RPC.
func New() *Client {
	return &Client{
		Addr:     config.GRPCAddr(),
		Token:    config.Token(),
		Insecure: config.GRPCInsecure(),
	}
}

// Close releases the underlying connection if one was opened.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.rpc = nil
	return err
}

// APIError wraps a gRPC error so existing callers can pattern-match by HTTP
// status. We map the most-used codes back to their HTTP equivalents.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("mgt-be %d: %s", e.Status, e.Body)
}

// dial returns a cached gRPC stub, creating the connection on first use.
func (c *Client) dial() (mgtv1.MgtServiceClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rpc != nil {
		return c.rpc, nil
	}
	if c.Addr == "" {
		return nil, errors.New("server gRPC addr not configured (run `mgt config set server_grpc_addr ...`)")
	}
	var opt grpc.DialOption
	if c.Insecure {
		opt = grpc.WithTransportCredentials(insecure.NewCredentials())
	} else {
		opt = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
	}
	conn, err := grpc.NewClient(c.Addr, opt)
	if err != nil {
		return nil, fmt.Errorf("dial mgt-be: %w", err)
	}
	c.conn = conn
	c.rpc = mgtv1.NewMgtServiceClient(conn)
	return c.rpc, nil
}

// withAuth adds the bearer token (when set) and a 30-second deadline.
func (c *Client) withAuth(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancelDeadline := context.WithTimeout(ctx, 30*time.Second)
		ctx = c.attachToken(ctx)
		return ctx, cancelDeadline
	}
	return c.attachToken(ctx), func() {}
}

func (c *Client) attachToken(ctx context.Context) context.Context {
	if c.Token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.Token)
}

// translate maps a gRPC status into an APIError-shaped value when possible.
func translate(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.Unauthenticated:
		return errors.New("not authenticated; run `mgt login`")
	}
	httpStatus := map[codes.Code]int{
		codes.InvalidArgument:    400,
		codes.NotFound:           404,
		codes.AlreadyExists:      409,
		codes.PermissionDenied:   403,
		codes.FailedPrecondition: 412,
		codes.Unavailable:        502,
		codes.DeadlineExceeded:   504,
		codes.Internal:           500,
	}[st.Code()]
	if httpStatus == 0 {
		httpStatus = 500
	}
	return &APIError{Status: httpStatus, Body: st.Message()}
}

// ── Wire types (identical shape to the previous REST client) ─────────────

type Me struct {
	ID        int64
	Login     string
	AvatarURL string
}

type Repo struct {
	ID            int64
	Owner         string
	Name          string
	FullName      string
	DefaultBranch string
}

type StackBranch struct {
	ID       int64
	Name     string
	Parent   string
	Position int
}

type Stack struct {
	ID          int64
	TrunkBranch string
	Branches    []StackBranch
}

type RebaseStep struct {
	Branch string
	Onto   string
}

type RebasePlan struct {
	Steps []RebaseStep
}

type CreatePR struct {
	Branch string
	Base   string
	Title  string
	Body   string
	Draft  bool
}

type PullRequest struct {
	Number      int
	Title       string
	State       string
	Draft       bool
	HeadBranch  string
	BaseBranch  string
	HTMLURL     string
	UpdatedAt   time.Time
	AuthorLogin string
	CIState     string
	Merged      bool
}

// CreateStack / AddBranch / MoveBranch / EnqueueMerge are kept purely as
// argument structs for backwards-compat with cmd/ callers. The fields they
// expose mirror the proto request messages.
type CreateStack struct {
	RepoID int64
	Branch string
}

type AddBranch struct {
	Name   string
	Parent string
}

type MoveBranch struct {
	Parent string
}

type EnqueueMerge struct {
	PRNumber int
	Method   string
}

type MergeQueueEntry struct {
	ID         int64
	PRNumber   int
	State      string
	Position   int
	Attempts   int
	LastError  string
	EnqueuedAt time.Time
}

// ── Conversions: pb → public types ───────────────────────────────────────

func meFromPB(p *mgtv1.MeResponse) Me {
	return Me{ID: p.GetId(), Login: p.GetLogin(), AvatarURL: p.GetAvatarUrl()}
}

func repoFromPB(p *mgtv1.Repo) Repo {
	return Repo{
		ID: p.GetId(), Owner: p.GetOwner(), Name: p.GetName(),
		FullName: p.GetFullName(), DefaultBranch: p.GetDefaultBranch(),
	}
}

func stackFromPB(p *mgtv1.Stack) Stack {
	if p == nil {
		return Stack{}
	}
	out := Stack{ID: p.GetId(), TrunkBranch: p.GetTrunkBranch()}
	for _, b := range p.GetBranches() {
		out.Branches = append(out.Branches, StackBranch{
			ID: b.GetId(), Name: b.GetName(), Parent: b.GetParent(), Position: int(b.GetPosition()),
		})
	}
	return out
}

func prFromPB(p *mgtv1.PullRequest) PullRequest {
	out := PullRequest{
		Number: int(p.GetNumber()), Title: p.GetTitle(), State: p.GetState(), Draft: p.GetDraft(),
		HeadBranch: p.GetHeadBranch(), BaseBranch: p.GetBaseBranch(), HTMLURL: p.GetHtmlUrl(),
		AuthorLogin: p.GetAuthorLogin(), CIState: p.GetCiState(), Merged: p.GetMerged(),
	}
	if t := p.GetUpdatedAt(); t != nil {
		out.UpdatedAt = t.AsTime()
	}
	return out
}

func mqFromPB(p *mgtv1.MergeQueueEntry) MergeQueueEntry {
	out := MergeQueueEntry{
		ID: p.GetId(), PRNumber: int(p.GetPrNumber()), State: p.GetState(),
		Position: int(p.GetPosition()), Attempts: int(p.GetAttempts()), LastError: p.GetLastError(),
	}
	if t := p.GetEnqueuedAt(); t != nil {
		out.EnqueuedAt = t.AsTime()
	}
	return out
}

// ── API methods (mirror the previous REST surface) ───────────────────────

func (c *Client) Me() (Me, error) {
	r, err := c.dial()
	if err != nil {
		return Me{}, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.Me(ctx, &emptypb.Empty{})
	if err != nil {
		return Me{}, translate(err)
	}
	return meFromPB(resp), nil
}

func (c *Client) ListRepos() ([]Repo, error) {
	r, err := c.dial()
	if err != nil {
		return nil, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.ListRepos(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]Repo, 0, len(resp.GetRepos()))
	for _, rp := range resp.GetRepos() {
		out = append(out, repoFromPB(rp))
	}
	return out, nil
}

func (c *Client) SyncRepos() ([]Repo, error) {
	r, err := c.dial()
	if err != nil {
		return nil, err
	}
	// SyncRepos hits GitHub server-side; allow more time.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = c.attachToken(ctx)
	resp, err := r.SyncRepos(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]Repo, 0, len(resp.GetRepos()))
	for _, rp := range resp.GetRepos() {
		out = append(out, repoFromPB(rp))
	}
	return out, nil
}

func (c *Client) StackByBranch(repoID int64, branch string) (Stack, error) {
	r, err := c.dial()
	if err != nil {
		return Stack{}, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.StackByBranch(ctx, &mgtv1.StackByBranchRequest{RepoId: repoID, Branch: branch})
	if err != nil {
		return Stack{}, translate(err)
	}
	return stackFromPB(resp), nil
}

func (c *Client) ListStacks(repoID int64) ([]Stack, error) {
	r, err := c.dial()
	if err != nil {
		return nil, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.ListStacks(ctx, &mgtv1.ListStacksRequest{RepoId: repoID})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]Stack, 0, len(resp.GetStacks()))
	for _, s := range resp.GetStacks() {
		out = append(out, stackFromPB(s))
	}
	return out, nil
}

func (c *Client) CreateStack(repoID int64, branch string) (Stack, error) {
	r, err := c.dial()
	if err != nil {
		return Stack{}, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.CreateStack(ctx, &mgtv1.CreateStackRequest{RepoId: repoID, Branch: branch})
	if err != nil {
		return Stack{}, translate(err)
	}
	return stackFromPB(resp), nil
}

func (c *Client) AddBranch(repoID, stackID int64, in AddBranch) (StackBranch, error) {
	r, err := c.dial()
	if err != nil {
		return StackBranch{}, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.AddBranch(ctx, &mgtv1.AddBranchRequest{
		RepoId: repoID, StackId: stackID, Name: in.Name, Parent: in.Parent,
	})
	if err != nil {
		return StackBranch{}, translate(err)
	}
	return StackBranch{
		ID: resp.GetId(), Name: resp.GetName(), Parent: resp.GetParent(), Position: int(resp.GetPosition()),
	}, nil
}

func (c *Client) MoveBranch(repoID, stackID, branchID int64, in MoveBranch) (RebasePlan, error) {
	r, err := c.dial()
	if err != nil {
		return RebasePlan{}, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.MoveBranch(ctx, &mgtv1.MoveBranchRequest{
		RepoId: repoID, StackId: stackID, BranchId: branchID, Parent: in.Parent,
	})
	if err != nil {
		return RebasePlan{}, translate(err)
	}
	out := RebasePlan{Steps: make([]RebaseStep, 0, len(resp.GetSteps()))}
	for _, st := range resp.GetSteps() {
		out.Steps = append(out.Steps, RebaseStep{Branch: st.GetBranch(), Onto: st.GetOnto()})
	}
	return out, nil
}

func (c *Client) DeleteBranch(repoID, stackID, branchID int64) error {
	r, err := c.dial()
	if err != nil {
		return err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	_, err = r.DeleteBranch(ctx, &mgtv1.DeleteBranchRequest{
		RepoId: repoID, StackId: stackID, BranchId: branchID,
	})
	return translate(err)
}

func (c *Client) ListPRs(repoID int64, state, author, q string) ([]PullRequest, error) {
	r, err := c.dial()
	if err != nil {
		return nil, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.ListPullRequests(ctx, &mgtv1.ListPullRequestsRequest{
		RepoId: repoID, State: state, Author: author, Q: q,
	})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]PullRequest, 0, len(resp.GetPulls()))
	for _, p := range resp.GetPulls() {
		out = append(out, prFromPB(p))
	}
	return out, nil
}

func (c *Client) CreatePR(repoID int64, in CreatePR, withAI bool) (PullRequest, error) {
	r, err := c.dial()
	if err != nil {
		return PullRequest{}, err
	}
	// PR creation can hit the LLM + GitHub; give it room.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = c.attachToken(ctx)
	resp, err := r.CreatePullRequest(ctx, &mgtv1.CreatePullRequestRequest{
		RepoId: repoID, Branch: in.Branch, Base: in.Base, Title: in.Title,
		Body: in.Body, Draft: in.Draft, Ai: withAI,
	})
	if err != nil {
		return PullRequest{}, translate(err)
	}
	return prFromPB(resp), nil
}

func (c *Client) DescribePR(repoID int64, number int) (string, error) {
	r, err := c.dial()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = c.attachToken(ctx)
	resp, err := r.DescribePullRequest(ctx, &mgtv1.DescribePullRequestRequest{
		RepoId: repoID, Number: int32(number),
	})
	if err != nil {
		err = translate(err)
		var ae *APIError
		if errors.As(err, &ae) && ae.Status == 412 {
			return "", errors.New("AI not configured on server (set LLM_API_KEY)")
		}
		return "", err
	}
	return resp.GetBody(), nil
}

func (c *Client) ListMergeQueue(repoID int64) ([]MergeQueueEntry, error) {
	r, err := c.dial()
	if err != nil {
		return nil, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.ListMergeQueue(ctx, &mgtv1.ListMergeQueueRequest{RepoId: repoID})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]MergeQueueEntry, 0, len(resp.GetEntries()))
	for _, e := range resp.GetEntries() {
		out = append(out, mqFromPB(e))
	}
	return out, nil
}

func (c *Client) EnqueueMerge(repoID int64, number int, method string) (MergeQueueEntry, error) {
	r, err := c.dial()
	if err != nil {
		return MergeQueueEntry{}, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.EnqueueMerge(ctx, &mgtv1.EnqueueMergeRequest{
		RepoId: repoID, PrNumber: int32(number), Method: method,
	})
	if err != nil {
		return MergeQueueEntry{}, translate(err)
	}
	return mqFromPB(resp), nil
}

func (c *Client) CancelMerge(repoID int64, entryID int64) error {
	r, err := c.dial()
	if err != nil {
		return err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	_, err = r.CancelMerge(ctx, &mgtv1.CancelMergeRequest{RepoId: repoID, EntryId: entryID})
	return translate(err)
}

// ── Device-flow helpers (used by `mgt login`) ─────────────────────────────

type DeviceStart struct {
	UserCode        string
	State           string
	VerificationURL string
}

func (c *Client) DeviceStart() (DeviceStart, error) {
	r, err := c.dial()
	if err != nil {
		return DeviceStart{}, err
	}
	ctx, cancel := c.withAuth(context.Background())
	defer cancel()
	resp, err := r.DeviceStart(ctx, &emptypb.Empty{})
	if err != nil {
		return DeviceStart{}, translate(err)
	}
	return DeviceStart{
		UserCode:        resp.GetUserCode(),
		State:           resp.GetState(),
		VerificationURL: resp.GetVerificationUrl(),
	}, nil
}

// DevicePoll blocks for up to ~5 minutes (server polls in 60s windows).
func (c *Client) DevicePoll(state string) (string, error) {
	r, err := c.dial()
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
		resp, err := r.DevicePoll(ctx, &mgtv1.DevicePollRequest{State: state})
		cancel()
		if err == nil {
			return resp.GetToken(), nil
		}
		st, ok := status.FromError(err)
		if !ok {
			return "", err
		}
		switch st.Code() {
		case codes.DeadlineExceeded:
			// Server's 60s wait elapsed — try again until our outer deadline.
			continue
		case codes.Unavailable:
			// Transient transport issue — back off briefly.
			time.Sleep(2 * time.Second)
			continue
		default:
			return "", translate(err)
		}
	}
	return "", io.EOF
}

// FormatPRNumber pretty-prints "#123" or "" if zero.
func FormatPRNumber(n int) string {
	if n == 0 {
		return ""
	}
	return "#" + strconv.Itoa(n)
}
