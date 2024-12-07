package bot

import (
	"context"
	"github.com/webitel/chat_manager/logger"
	"log/slog"
)

func (srv *Service) LogAction(ctx context.Context, message *logger.Message) {
	err := srv.audit.SendContext(ctx, message)
	if err != nil {
		srv.Log.Error(err.Error(),
			slog.Any("error", err),
		)
	}
}
