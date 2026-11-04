// INPUT: An admitted round mutation and its owning runtime supervisor.
// OUTPUT: Optional native foreground eligibility for the owned runtime only.
// POS: Private OS handoff after approval; never changes target/session grants.
package computeruse

import "context"

func (r *Round) prepareForeground(ctx context.Context) {
	if r.service == nil {
		return
	}
	if supervisor, ok := r.service.supervisor.(interface{ prepareForeground(context.Context) }); ok {
		supervisor.prepareForeground(ctx)
	}
}

func (s *ProcessSupervisor) prepareForeground(ctx context.Context) {
	s.mu.Lock()
	process := s.process
	if process == nil {
		s.mu.Unlock()
		return
	}
	select {
	case <-process.done:
		s.mu.Unlock()
		return
	default:
	}
	owned := process.cmd.Process
	s.mu.Unlock()
	prepareNativeForeground(ctx, owned)
}
