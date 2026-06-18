package ctrl

import (
	"context"

	"github.com/postmannen/actress"
)

func etRemoteFn() actress.ETFunc {
	fn := func(ctx context.Context, p *actress.Process) func() {
		fn := func() {

		}
		return fn
	}
	return fn
}
