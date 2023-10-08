package share

import "context"

type UI interface {
	ChangeQuery(ctx context.Context, query string)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	ToggleApp(ctx context.Context)
	ShowMsg(ctx context.Context, title string, description string, icon string)
}
