// Package core holds the shared dependencies used by both the REST (api) and
// gRPC (grpcserver) layers. Bundling them lets us guarantee that both
// servers see the same DeviceFlow / queue-notifier / db pool.
package core

import (
	"github.com/santhoshrox/mgt-be/internal/ai"
	"github.com/santhoshrox/mgt-be/internal/auth"
	"github.com/santhoshrox/mgt-be/internal/config"
	"github.com/santhoshrox/mgt-be/internal/crypto"
	"github.com/santhoshrox/mgt-be/internal/db"
	"github.com/santhoshrox/mgt-be/internal/search"
)

// QueueController is implemented by the merge-queue worker. Both servers
// nudge it after enqueue/cancel so the worker can pick up changes
// immediately instead of waiting for the next poll.
type QueueController interface {
	Notify(repoID int64)
}

type Core struct {
	Cfg    *config.Config
	DB     *db.DB
	Sealer *crypto.Sealer
	Signer *auth.SessionSigner
	Device *auth.DeviceFlow
	AI     *ai.Client
	Search *search.Client
	Queue  QueueController
}

func New(
	cfg *config.Config,
	dbConn *db.DB,
	sealer *crypto.Sealer,
	signer *auth.SessionSigner,
	aiClient *ai.Client,
	searchClient *search.Client,
	queue QueueController,
) *Core {
	return &Core{
		Cfg:    cfg,
		DB:     dbConn,
		Sealer: sealer,
		Signer: signer,
		Device: auth.NewDeviceFlow(),
		AI:     aiClient,
		Search: searchClient,
		Queue:  queue,
	}
}
